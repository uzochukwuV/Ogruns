#!/usr/bin/env python3
"""
AI Trading Signal Agent Broadcaster
Scans CoinGecko for market movers, generates reversal signals,
signs them with EIP-191, and posts to the Verifier API.

Usage:
    pip install -r requirements.txt
    cp .env.example .env
    # Edit .env with your AGENT_PRIVATE_KEY
    python broadcaster.py

    # One-shot scan (no loop):
    python broadcaster.py --once

    # Custom thresholds:
    python broadcaster.py --min 5 --max 8 --interval 120
"""

import argparse
import json
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
from typing import Dict, List, Optional
from dataclasses import dataclass, asdict

import requests

from config import Config
from scanner import (
    fetch_market_movers,
    get_coin_data,
    fetch_ohlc_data,
    detect_candlestick_patterns,
)
from generator import generate_signal, compute_reversal_score, SignalPayload
from signer import create_envelope


@dataclass
class DelayedAnalysis:
    coin_id: str
    token_pair: str
    scheduled_at: int
    ohlc_raw: List
    pattern_analysis: Dict
    initial_score: int
    tier: str  # "mild" or "moderate"


delayed_queue: List[DelayedAnalysis] = []
sent_signals: Dict[str, int] = {}


class C:
    RESET = "\033[0m"
    BOLD = "\033[1m"
    GREEN = "\033[92m"
    RED = "\033[91m"
    YELLOW = "\033[93m"
    CYAN = "\033[96m"
    GRAY = "\033[90m"
    WHITE = "\033[97m"


def log(msg: str, level: str = "info"):
    ts = datetime.now().strftime("%H:%M:%S")
    colors = {
        "info": C.GRAY,
        "success": C.GREEN,
        "warn": C.YELLOW,
        "error": C.RED,
        "signal": C.GREEN + C.BOLD,
        "header": C.CYAN + C.BOLD,
        "schedule": C.CYAN,
    }
    color = colors.get(level, C.RESET)
    print(f"{C.GRAY}[{ts}]{C.RESET} {color}{msg}{C.RESET}")


def submit_signal(envelope: dict) -> dict:
    """Submit a signed signal to the Verifier API"""
    url = f"{Config.VERIFIER_API_URL}/api/v1/signals"
    body = {
        "node_id": envelope["node_id"],
        "envelope": envelope,
    }

    response = requests.post(url, json=body, timeout=10)
    response.raise_for_status()
    return response.json()


def load_queue() -> List[DelayedAnalysis]:
    """Load delayed queue from JSON file."""
    global delayed_queue
    try:
        with open(Config.QUEUE_FILE, "r") as f:
            data = json.load(f)
            return [DelayedAnalysis(**item) for item in data]
    except (FileNotFoundError, json.JSONDecodeError):
        return []


def save_queue():
    """Persist delayed queue to JSON file."""
    with open(Config.QUEUE_FILE, "w") as f:
        json.dump([asdict(item) for item in delayed_queue], f, indent=2)


def load_sent_signals() -> Dict[str, int]:
    """Load recently submitted signal keys from disk."""
    try:
        with open(Config.SENT_SIGNALS_FILE, "r") as f:
            data = json.load(f)
            return {str(k): int(v) for k, v in data.items()}
    except (FileNotFoundError, json.JSONDecodeError, TypeError, ValueError):
        return {}


def save_sent_signals():
    """Persist recently submitted signal keys after pruning old entries."""
    cutoff = int(time.time()) - Config.SIGNAL_COOLDOWN_SEC
    pruned = {k: ts for k, ts in sent_signals.items() if ts >= cutoff}
    with open(Config.SENT_SIGNALS_FILE, "w") as f:
        json.dump(pruned, f, indent=2, sort_keys=True)


def signal_key(signal: SignalPayload) -> str:
    """Stable duplicate key for the cooldown window."""
    return f"{signal.token_pair}:{signal.direction}"


def is_recent_duplicate(signal: SignalPayload) -> bool:
    last_sent = sent_signals.get(signal_key(signal))
    if not last_sent:
        return False
    return int(time.time()) - last_sent < Config.SIGNAL_COOLDOWN_SEC


def remember_signal(signal: SignalPayload):
    sent_signals[signal_key(signal)] = int(time.time())
    save_sent_signals()


def has_pending_delayed(token_pair: str) -> bool:
    return any(item.token_pair == token_pair for item in delayed_queue)


def schedule_delayed_analysis(item: DelayedAnalysis):
    if has_pending_delayed(item.token_pair):
        log(f"Skipping duplicate delayed analysis for {item.token_pair}", "warn")
        return
    delayed_queue.append(item)


def run_scan(min_pct: float, max_pct: float, min_score: int = 55) -> int:
    """Run a single scan cycle. Returns the number of signals submitted."""
    global delayed_queue, sent_signals
    delayed_queue = load_queue()
    sent_signals = load_sent_signals()
    current_time = int(time.time())

    log(f"Scanning for coins with {min_pct}-{max_pct}% 24h change...", "info")

    try:
        movers = fetch_market_movers(min_pct=min_pct, max_pct=max_pct)
    except requests.HTTPError as e:
        if e.response.status_code == 429:
            log("Rate limited by CoinGecko. Waiting 60s...", "warn")
            time.sleep(60)
            return 0
        log(f"CoinGecko error: {e}", "error")
        return 0
    except Exception as e:
        log(f"Fetch error: {e}", "error")
        return 0

    if not movers:
        log(f"No coins found in {min_pct}-{max_pct}% range", "warn")
        return 0

    log(f"Found {len(movers)} coins. Fetching OHLC data...", "info")

    # Volume filter + parallel OHLC fetch
    qualified = []
    for coin in movers:
        data = get_coin_data(coin)
        if data["volume_24h"] < Config.MIN_VOLUME_USD:
            log(f"Skipping {data['symbol']} (volume ${data['volume_24h']:,.0f} < ${Config.MIN_VOLUME_USD:,.0f})", "info")
            continue
        qualified.append((coin, data))

    # Parallel OHLC fetch
    scored = []
    ohlc_success = 0
    with ThreadPoolExecutor(max_workers=Config.MAX_OHLC_WORKERS) as executor:
        futures = {
            executor.submit(fetch_ohlc_data, coin["id"], silent=True): (coin, data)
            for coin, data in qualified
        }

        for future in as_completed(futures):
            coin, data = futures[future]
            ohlc = future.result()
            if ohlc:
                ohlc_success += 1
            else:
                log(f"No OHLC candles returned for {data['symbol']}", "warn")
            pattern_analysis = detect_candlestick_patterns(ohlc)
            data["_ohlc_raw"] = ohlc
            data["_pattern_analysis"] = pattern_analysis

            analysis = compute_reversal_score(data)
            data["_analysis"] = analysis
            scored.append(data)

    log(f"Fetched OHLC candles for {ohlc_success}/{len(qualified)} qualified coin(s)", "info")

    scored.sort(key=lambda x: x["_analysis"]["score"], reverse=True)

    print(f"\n{C.GRAY}{'SYMBOL':<10} {'PRICE':>12} {'24H%':>8} {'VOL':>10} {'SCORE':>6} {'PAT':>8}  {'STATUS'}{C.RESET}")
    print(C.GRAY + "-" * 75 + C.RESET)

    for data in scored[:10]:
        analysis = data["_analysis"]
        score = analysis["score"]
        patterns = analysis.get("patterns", [])
        pattern_names = [p["name"][:3].upper() for p in patterns[:3]]
        pattern_str = ",".join(pattern_names) if pattern_names else "-"

        if score >= 90:
            status = f"{C.GREEN + C.BOLD}VERY STRONG{C.RESET}"
        elif score >= 80:
            status = f"{C.GREEN}STRONG{C.RESET}"
        elif score >= Config.MODERATE_LOWER:
            status = f"{C.YELLOW}MODERATE{C.RESET}"
        elif score >= Config.MILD_LOWER:
            status = f"{C.CYAN}MILD{C.RESET}"
        elif score >= Config.WATCH_LOWER:
            status = f"{C.WHITE}WATCH{C.RESET}"
        else:
            status = f"{C.GRAY}WEAK{C.RESET}"

        change_color = C.GREEN if data["change_24h"] > 0 else C.RED
        print(
            f"{C.WHITE}{data['symbol']:<10}{C.RESET}"
            f"${data['price']:>11.2f} "
            f"{change_color}{data['change_24h']:>+7.2f}%{C.RESET} "
            f"{data['volume_24h']/1e6:>8.1f}M "
            f"{score:>5} {pattern_str:>8}  {status}"
        )

    print()

    process_delayed_analysis(min_score)

    signals_submitted = 0

    for data in scored:
        analysis = data["_analysis"]
        score = analysis["score"]

        if score >= 90:
            signal = generate_signal(data, min_score=min_score)
            if signal:
                if is_recent_duplicate(signal):
                    log(f"Skipping duplicate {signal.direction.upper()} {signal.token_pair} within cooldown", "warn")
                    continue
                envelope = create_envelope(Config.AGENT_PRIVATE_KEY, signal)
                try:
                    log(f"Submitting HIGH-PRIORITY signal for {data['token_pair']}", "signal")
                    result = submit_signal(envelope)
                    log(f"  -> Queued: {result.get('signal_id', 'unknown')}", "success")
                    remember_signal(signal)
                    signals_submitted += 1
                except Exception as e:
                    log(f"  -> Error: {e}", "error")
                time.sleep(1.1)
        elif score >= 80:
            signal = generate_signal(data, min_score=min_score)
            if signal:
                if submit_signal_with_logging(signal):
                    signals_submitted += 1
                time.sleep(1.1)
        elif Config.MILD_LOWER <= score < Config.MODERATE_LOWER:
            # MILD tier - 15 min delayed analysis
            schedule_delayed_analysis(DelayedAnalysis(
                coin_id=data["id"],
                token_pair=data["token_pair"],
                scheduled_at=current_time + Config.DELAYED_ANALYSIS_MILD_DELAY,
                ohlc_raw=data.get("_ohlc_raw", []),
                pattern_analysis=data["_pattern_analysis"],
                initial_score=score,
                tier="mild",
            ))
            if has_pending_delayed(data["token_pair"]):
                log(f"Scheduled {data['symbol']} for MILD delayed analysis (score: {score})", "schedule")
        elif Config.MODERATE_LOWER <= score < Config.STRONG_LOWER:
            # MODERATE tier - 30 min delayed analysis
            schedule_delayed_analysis(DelayedAnalysis(
                coin_id=data["id"],
                token_pair=data["token_pair"],
                scheduled_at=current_time + Config.DELAYED_ANALYSIS_MODERATE_DELAY,
                ohlc_raw=data.get("_ohlc_raw", []),
                pattern_analysis=data["_pattern_analysis"],
                initial_score=score,
                tier="moderate",
            ))
            if has_pending_delayed(data["token_pair"]):
                log(f"Scheduled {data['symbol']} for MODERATE delayed analysis (score: {score})", "schedule")

    save_queue()
    return signals_submitted


def process_delayed_analysis(min_score: int) -> None:
    """Process tokens in the delayed analysis queue."""
    global delayed_queue

    if not delayed_queue:
        return

    log(f"Processing {len(delayed_queue)} delayed analysis items...", "info")

    remaining = []
    for item in delayed_queue:
        if int(time.time()) >= item.scheduled_at:
            try:
                ohlc = fetch_ohlc_data(item.coin_id)
                new_patterns = detect_candlestick_patterns(ohlc)

                url = f"{Config.COINGECKO_BASE_URL}/coins/markets"
                params = {"vs_currency": "usd", "ids": item.coin_id}
                headers = {}
                if Config.COINGECKO_API_KEY:
                    headers["x-cg-demo-api-key"] = Config.COINGECKO_API_KEY
                response = requests.get(url, params=params, headers=headers, timeout=15)
                response.raise_for_status()
                coins = response.json()

                if coins:
                    data = get_coin_data(coins[0])
                    data["_pattern_analysis"] = new_patterns

                    analysis = compute_reversal_score(data)
                    score = analysis["score"]

                    if score >= 65:
                        signal = generate_signal(data, min_score=min_score)
                        if signal:
                            submit_signal_with_logging(signal)
                            log(f"Delayed {item.tier} signal for {item.token_pair}: score {score}", "success")
                    else:
                        log(f"Delayed analysis {item.token_pair}: score {score} (no signal)", "info")
            except Exception as e:
                log(f"Delayed analysis error for {item.token_pair}: {e}", "error")
                remaining.append(item)
        else:
            remaining.append(item)

    delayed_queue = remaining
    save_queue()


def submit_signal_with_logging(signal: SignalPayload) -> bool:
    """Submit a signal with logging."""
    if is_recent_duplicate(signal):
        log(f"Skipping duplicate {signal.direction.upper()} {signal.token_pair} within cooldown", "warn")
        return False

    try:
        envelope = create_envelope(Config.AGENT_PRIVATE_KEY, signal)
        expiry_mins = (signal.expiry_time - int(time.time())) // 60

        log(
            f"Submitting {signal.direction.upper()} {signal.token_pair} "
            f"(score: {signal.weight_pct}%, expiry: {expiry_mins}m)",
            "signal"
        )

        result = submit_signal(envelope)
        status = result.get("status", "ok")

        if status == "queued":
            signal_id = result.get("signal_id", "unknown")
            log(f"  -> Queued: {signal_id}", "success")
            remember_signal(signal)
            return True
        else:
            log(f"  -> Response: {result}", "warn")
    except requests.HTTPError as e:
        try:
            err_body = e.response.json()
            log(f"  -> Rejected: {err_body.get('error', str(e))}", "error")
        except Exception:
            log(f"  -> API error ({e.response.status_code}): {e}", "error")
    except Exception as e:
        log(f"  -> Error: {e}", "error")

    return False


def main():
    parser = argparse.ArgumentParser(description="AI Trading Signal Agent")
    parser.add_argument("--min", type=float, default=Config.MIN_PRICE_CHANGE_PCT)
    parser.add_argument("--max", type=float, default=Config.MAX_PRICE_CHANGE_PCT)
    parser.add_argument("--interval", type=int, default=Config.SCAN_INTERVAL_SEC)
    parser.add_argument("--min-score", type=int, default=55)
    parser.add_argument("--once", action="store_true")
    args = parser.parse_args()

    if not Config.validate():
        sys.exit(1)

    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    from eth_account import Account
    account = Account.from_key(
        Config.AGENT_PRIVATE_KEY if Config.AGENT_PRIVATE_KEY.startswith("0x")
        else "0x" + Config.AGENT_PRIVATE_KEY
    )

    print()
    print(C.CYAN + C.BOLD + "  ╔═══════════════════════════════════════════════════════╗" + C.RESET)
    print(C.CYAN + C.BOLD + "  ║     0G SIGNAL INTELLIGENCE NETWORK - AGENT            ║" + C.RESET)
    print(C.CYAN + C.BOLD + "  ╚═══════════════════════════════════════════════════════╝" + C.RESET)
    print()
    print(f"  {C.WHITE}Agent ID :{C.RESET} {account.address}")
    print(f"  {C.WHITE}API      :{C.RESET} {Config.VERIFIER_API_URL}")
    print(f"  {C.WHITE}Filter   :{C.RESET} {args.min}% - {args.max}% price change (24h)")
    print(f"  {C.WHITE}Min Score:{C.RESET} {args.min_score}")
    print(f"  {C.WHITE}Interval :{C.RESET} {args.interval}s")
    print(f"  {C.WHITE}Mode     :{C.RESET} {'One-shot' if args.once else 'Continuous loop'}")
    print()

    if args.once:
        run_scan(args.min, args.max, args.min_score)
        return

    scan_count = 0
    try:
        while True:
            scan_count += 1
            log(f"=== Scan #{scan_count} ===", "header")

            signals = run_scan(args.min, args.max, args.min_score)
            log(f"Submitted {signals} signal(s)", "info")

            log(f"Next scan in {args.interval}s. Press Ctrl+C to stop.", "info")
            time.sleep(args.interval)

    except KeyboardInterrupt:
        print()
        log("Agent stopped.", "warn")
        save_queue()


if __name__ == "__main__":
    main()
