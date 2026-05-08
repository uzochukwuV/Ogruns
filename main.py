#!/usr/bin/env python3
"""
DRIP Signal Broadcaster
=======================
Scans CoinGecko /coins/markets for coins moving 4-9% (bull or bear),
scores reversal probability using momentum analysis, then broadcasts
AI-generated signals via Claude Sonnet.

Usage:
    pip install requests anthropic
    python signal_broadcaster.py

    # Custom range + interval:
    python signal_broadcaster.py --min 5 --max 8 --interval 120

    # One-shot scan (no loop):
    python signal_broadcaster.py --once
"""

import argparse
import json
import math
import os
import sys
import time
from datetime import datetime
from typing import Optional

import anthropic
import requests

# ─── CONFIG ────────────────────────────────────────────────────────────────
COINGECKO_BASE = "https://api.coingecko.com/api/v3"
DEFAULT_MIN_PCT = 4.0
DEFAULT_MAX_PCT = 9.0
DEFAULT_INTERVAL_SEC = 90
PER_PAGE = 250

# Colors for terminal output
class C:
    RESET  = "\033[0m"
    BOLD   = "\033[1m"
    GREEN  = "\033[92m"
    RED    = "\033[91m"
    YELLOW = "\033[93m"
    CYAN   = "\033[96m"
    GRAY   = "\033[90m"
    WHITE  = "\033[97m"
    BG_DARK = "\033[40m"


# ─── LOGGING ───────────────────────────────────────────────────────────────
def log(msg: str, level: str = "info"):
    ts = datetime.now().strftime("%H:%M:%S")
    colors = {
        "info":    C.GRAY,
        "success": C.GREEN,
        "warn":    C.YELLOW,
        "error":   C.RED,
        "buy":     C.GREEN + C.BOLD,
        "sell":    C.RED + C.BOLD,
        "hold":    C.YELLOW,
        "header":  C.CYAN + C.BOLD,
        "signal":  C.WHITE + C.BOLD,
    }
    color = colors.get(level, C.RESET)
    print(f"{C.GRAY}[{ts}]{C.RESET} {color}{msg}{C.RESET}")


def divider(char="─", width=72):
    print(C.GRAY + char * width + C.RESET)


# ─── REVERSAL SCORING ──────────────────────────────────────────────────────
def compute_reversal_score(coin: dict) -> dict:
    h1  = coin.get("price_change_percentage_1h_in_currency") or 0.0
    h24 = coin.get("price_change_percentage_24h_in_currency") or 0.0
    d7  = coin.get("price_change_percentage_7d_in_currency") or 0.0
    vol  = coin.get("total_volume") or 0
    mcap = coin.get("market_cap") or 1
    price = coin.get("current_price") or 0
    ath   = coin.get("ath") or price or 1
    atl   = coin.get("atl") or 0

    direction = "BULL" if h24 >= 0 else "BEAR"

    # Momentum decay: is the 1h pace slowing vs 24h average hourly pace?
    avg_hourly = h24 / 24
    momentum_decay = (
        (h1 < avg_hourly) if direction == "BULL"
        else (h1 > avg_hourly)
    )

    # Volume spike: daily vol > 15% of market cap
    vol_ratio = vol / mcap
    vol_spike = vol_ratio > 0.15

    # Timeframe divergence: not all 1h/24h/7d pointing same way
    signs = [math.copysign(1, x) for x in [h1, h24, d7] if x != 0]
    all_aligned = len(set(signs)) == 1 if signs else False

    # Overextension
    overextended = abs(h24) > 7

    # ATH proximity (for bull moves — near ATH means less reversal room)
    ath_pct = ((price - ath) / ath * 100) if ath > 0 else 0

    # Score
    score = 50
    if momentum_decay:  score += 20
    if vol_spike:       score += 15
    if not all_aligned: score += 15
    if overextended:    score += 10
    if direction == "BULL" and ath_pct > -10:  score -= 10
    score = max(0, min(100, score))

    reversal_dir = "BEAR REVERSAL" if direction == "BULL" else "BULL REVERSAL"

    return {
        "score": score,
        "direction": direction,
        "reversal_dir": reversal_dir,
        "momentum_decay": momentum_decay,
        "vol_spike": vol_spike,
        "all_aligned": all_aligned,
        "overextended": overextended,
        "vol_ratio": round(vol_ratio * 100, 1),
    }


def signal_label(score: int) -> tuple[str, str]:
    if score >= 75: return "STRONG SIGNAL",   C.GREEN + C.BOLD
    if score >= 60: return "MODERATE SIGNAL", C.YELLOW + C.BOLD
    if score >= 45: return "WEAK SIGNAL",     C.YELLOW
    return "NO SIGNAL", C.GRAY


# ─── COINGECKO FETCH ───────────────────────────────────────────────────────
def fetch_market_movers(min_pct: float, max_pct: float) -> list[dict]:
    log(f"Fetching /coins/markets (top {PER_PAGE} by market cap)...", "info")
    params = {
        "vs_currency": "usd",
        "order": "market_cap_desc",
        "per_page": PER_PAGE,
        "page": 1,
        "price_change_percentage": "1h,24h,7d",
        "sparkline": "false",
    }
    resp = requests.get(f"{COINGECKO_BASE}/coins/markets", params=params, timeout=30)
    resp.raise_for_status()
    all_coins = resp.json()

    filtered = [
        c for c in all_coins
        if min_pct <= abs(c.get("price_change_percentage_24h_in_currency") or 0) <= max_pct
    ]
    return filtered


# ─── CLAUDE AI ANALYSIS ────────────────────────────────────────────────────
def analyze_with_claude(coins: list[dict], client: anthropic.Anthropic) -> list[dict]:
    top = coins[:8]
    summary = []
    for c in top:
        analysis = compute_reversal_score(c)
        summary.append({
            "symbol": c["symbol"].upper(),
            "price": c.get("current_price"),
            "change_1h":  round(c.get("price_change_percentage_1h_in_currency") or 0, 2),
            "change_24h": round(c.get("price_change_percentage_24h_in_currency") or 0, 2),
            "change_7d":  round(c.get("price_change_percentage_7d_in_currency") or 0, 2),
            "vol_mcap_ratio_pct": analysis["vol_ratio"],
            "reversal_score": analysis["score"],
            "momentum_decay": analysis["momentum_decay"],
            "vol_spike": analysis["vol_spike"],
        })

    prompt = f"""You are a crypto trading signal analyst. Analyze these coins that have moved {DEFAULT_MIN_PCT}-{DEFAULT_MAX_PCT}% and are showing potential reversal signals.

Coins data:
{json.dumps(summary, indent=2)}

For each coin with reversal_score >= 55, provide:
1. SIGNAL: BUY / SELL / HOLD
2. CONFIDENCE: High / Medium / Low
3. REASON: 1 sentence max (momentum, volume, or divergence based)
4. RISK: 1 key risk factor
5. TARGET_PCT: estimated % move if signal plays out (e.g. 3.5)

Return ONLY a valid JSON array, no markdown, no preamble:
[{{"symbol": "BTC", "signal": "BUY", "confidence": "High", "reason": "...", "risk": "...", "target_pct": 3.5}}]

Only include coins worth acting on (score >= 55). Max 5 results."""

    log("Sending top movers to Claude Sonnet for AI analysis...", "info")
    message = client.messages.create(
        model="claude-sonnet-4-20250514",
        max_tokens=1000,
        messages=[{"role": "user", "content": prompt}],
    )
    raw = message.content[0].text.strip()

    # Safe parse
    start = raw.find("[")
    end   = raw.rfind("]")
    if start == -1:
        log("Claude returned no parseable signals.", "warn")
        return []
    return json.loads(raw[start:end + 1])


# ─── DISPLAY ───────────────────────────────────────────────────────────────
def display_coins(coins: list[dict]):
    divider()
    header = f"{'SYMBOL':<10} {'PRICE':>12} {'1H%':>7} {'24H%':>7} {'7D%':>7}  {'SCORE':>5}  {'STATUS':<22} {'FLAGS'}"
    print(C.GRAY + header + C.RESET)
    divider("·")

    for coin in coins:
        sym   = coin["symbol"].upper()
        price = coin.get("current_price", 0)
        h1    = coin.get("price_change_percentage_1h_in_currency") or 0
        h24   = coin.get("price_change_percentage_24h_in_currency") or 0
        d7    = coin.get("price_change_percentage_7d_in_currency") or 0
        a     = coin["_analysis"]
        label, color = signal_label(a["score"])

        def pct_str(v):
            c = C.GREEN if v > 0 else C.RED if v < 0 else C.GRAY
            return f"{c}{v:+.2f}%{C.RESET}"

        price_str = f"${price:.6f}" if price < 0.01 else f"${price:.4f}" if price < 1 else f"${price:.2f}"

        flags = []
        if a["momentum_decay"]: flags.append("⟂decay")
        if a["vol_spike"]:      flags.append("⬆vol")
        if a["overextended"]:   flags.append("⚡ext")
        if not a["all_aligned"]: flags.append("⊘div")
        flags_str = " ".join(flags)

        print(
            f"{C.WHITE}{sym:<10}{C.RESET}"
            f"{price_str:>15}  "
            f"{pct_str(h1):>18}  "
            f"{pct_str(h24):>18}  "
            f"{pct_str(d7):>18}  "
            f"{color}{a['score']:>3}{C.RESET}  "
            f"{color}{label:<22}{C.RESET}"
            f"{C.GRAY}{flags_str}{C.RESET}"
        )

    divider()


def display_ai_signals(signals: list[dict]):
    if not signals:
        log("No actionable signals this scan.", "warn")
        return

    divider("═")
    print(f"{C.CYAN}{C.BOLD}  📡  AI SIGNAL BROADCAST  —  {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}{C.RESET}")
    divider("═")

    for s in signals:
        sig = s.get("signal", "HOLD")
        conf = s.get("confidence", "Low")
        reason = s.get("reason", "")
        risk   = s.get("risk", "")
        target = s.get("target_pct", "?")

        if sig == "BUY":
            sig_color = C.GREEN + C.BOLD
            arrow = "▲"
        elif sig == "SELL":
            sig_color = C.RED + C.BOLD
            arrow = "▼"
        else:
            sig_color = C.YELLOW + C.BOLD
            arrow = "◆"

        conf_color = C.GREEN if conf == "High" else C.YELLOW if conf == "Medium" else C.GRAY

        print()
        print(f"  {C.WHITE}{C.BOLD}{s['symbol']:<8}{C.RESET}  {sig_color}{arrow} {sig:<5}{C.RESET}  {conf_color}{conf} confidence{C.RESET}  {C.CYAN}Target: {target}%{C.RESET}")
        print(f"  {C.GRAY}Reason :{C.RESET} {reason}")
        print(f"  {C.GRAY}Risk   :{C.RESET} {C.YELLOW}{risk}{C.RESET}")

    print()
    divider("═")


# ─── MAIN SCAN LOOP ────────────────────────────────────────────────────────
def run_scan(min_pct: float, max_pct: float, client: anthropic.Anthropic):
    print()
    log(f"🔍 Scanning for coins with {min_pct}–{max_pct}% move (24h)...", "header")

    try:
        coins = fetch_market_movers(min_pct, max_pct)
    except requests.HTTPError as e:
        if e.response.status_code == 429:
            log("Rate limited by CoinGecko. Waiting 60s before retry...", "warn")
            time.sleep(60)
            return
        log(f"CoinGecko error: {e}", "error")
        return
    except Exception as e:
        log(f"Fetch error: {e}", "error")
        return

    if not coins:
        log(f"No coins found in {min_pct}–{max_pct}% range this scan.", "warn")
        return

    # Score and sort
    for c in coins:
        c["_analysis"] = compute_reversal_score(c)
    coins.sort(key=lambda c: c["_analysis"]["score"], reverse=True)

    log(f"Found {len(coins)} coins. Top by reversal score:", "success")
    display_coins(coins[:20])  # show top 20

    # Claude analysis
    try:
        ai_signals = analyze_with_claude(coins, client)
        display_ai_signals(ai_signals)
    except Exception as e:
        log(f"Claude analysis error: {e}", "error")


def main():
    parser = argparse.ArgumentParser(description="Drip Signal Broadcaster")
    parser.add_argument("--min",      type=float, default=DEFAULT_MIN_PCT,     help="Min price change %% (default: 4)")
    parser.add_argument("--max",      type=float, default=DEFAULT_MAX_PCT,     help="Max price change %% (default: 9)")
    parser.add_argument("--interval", type=int,   default=DEFAULT_INTERVAL_SEC, help="Scan interval in seconds (default: 90)")
    parser.add_argument("--once",     action="store_true",                      help="Run one scan and exit")
    parser.add_argument("--api-key",  type=str,   default=None,                help="Anthropic API key (or set ANTHROPIC_API_KEY env var)")
    args = parser.parse_args()

    api_key = args.api_key or os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        print(f"{C.RED}Error: ANTHROPIC_API_KEY not set.{C.RESET}")
        print("Set it via:  export ANTHROPIC_API_KEY=sk-ant-...")
        print("Or pass:     --api-key sk-ant-...")
        sys.exit(1)

    client = anthropic.Anthropic(api_key=api_key)

    # Banner
    print()
    print(C.CYAN + C.BOLD + "  ██████╗ ██████╗ ██╗██████╗     ███████╗██╗ ██████╗ ███╗   ██╗ █████╗ ██╗" + C.RESET)
    print(C.CYAN         + "  ██╔══██╗██╔══██╗██║██╔══██╗    ██╔════╝██║██╔════╝ ████╗  ██║██╔══██╗██║" + C.RESET)
    print(C.CYAN         + "  ██║  ██║██████╔╝██║██████╔╝    ███████╗██║██║  ███╗██╔██╗ ██║███████║██║" + C.RESET)
    print(C.CYAN         + "  ██║  ██║██╔══██╗██║██╔═══╝     ╚════██║██║██║   ██║██║╚██╗██║██╔══██║██║" + C.RESET)
    print(C.CYAN + C.BOLD + "  ██████╔╝██║  ██║██║██║         ███████║██║╚██████╔╝██║ ╚████║██║  ██║███████╗" + C.RESET)
    print(C.CYAN         + "  ╚═════╝ ╚═╝  ╚═╝╚═╝╚═╝         ╚══════╝╚═╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝" + C.RESET)
    print(C.GRAY + "  BROADCASTER  ·  CoinGecko /coins/markets  ·  Claude Sonnet AI Analysis" + C.RESET)
    print()
    print(f"  {C.WHITE}Filter  :{C.RESET} {args.min}% – {args.max}% price change (24h)")
    print(f"  {C.WHITE}Interval:{C.RESET} {args.interval}s")
    print(f"  {C.WHITE}Mode    :{C.RESET} {'One-shot' if args.once else 'Continuous loop'}")
    print()

    if args.once:
        run_scan(args.min, args.max, client)
        return

    scan_count = 0
    try:
        while True:
            scan_count += 1
            log(f"Scan #{scan_count}", "header")
            run_scan(args.min, args.max, client)

            log(f"Next scan in {args.interval}s. Press Ctrl+C to stop.", "info")
            time.sleep(args.interval)

    except KeyboardInterrupt:
        print()
        log("Broadcaster stopped.", "warn")


if __name__ == "__main__":
    main()