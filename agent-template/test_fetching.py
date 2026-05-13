#!/usr/bin/env python3
"""
Test script for data fetching tools.
Tests historical price fetching, signal history, and time analysis.
"""
import requests
import time
from datetime import datetime, timezone, timedelta
from typing import List, Dict, Optional
from dotenv import load_dotenv
import os

load_dotenv()

COINGECKO_API_KEY = os.getenv("COINGECKO_API_KEY", "")
VERIFIER_API_URL = os.getenv("VERIFIER_API_URL", "http://localhost:8080")

# Token pair to CoinGecko ID mapping
PAIR_TO_COINGECKO = {
    "BTC/USDT": "bitcoin",
    "ETH/USDT": "ethereum",
    "SOL/USDT": "solana",
    "DOGE/USDT": "dogecoin",
    "XRP/USDT": "ripple",
    "ADA/USDT": "cardano",
    "AVAX/USDT": "avalanche-2",
    "LINK/USDT": "chainlink",
    "DOT/USDT": "polkadot",
    "MATIC/USDT": "matic-network",
    "UNI/USDT": "uniswap",
    "LTC/USDT": "litecoin",
    "ATOM/USDT": "cosmos",
    "NEAR/USDT": "near",
    "ICP/USDT": "internet-computer",
    "FIL/USDT": "filecoin",
    "AAVE/USDT": "aave",
    "ARB/USDT": "arbitrum",
    "APT/USDT": "aptos",
    "OP/USDT": "optimism",
    "SUI/USDT": "sui",
    "HBAR/USDT": "hedera-hashgraph",
    "XLM/USDT": "stellar",
    "ALGO/USDT": "algorand",
    "PEPE/USDT": "pepe",
    "SHIB/USDT": "shiba-inu",
    "TRX/USDT": "tron",
    "INJ/USDT": "injective-protocol",
    "FET/USDT": "fetch-ai",
    "ONDO/USDT": "ondo-finance",
    "SEI/USDT": "sei-network",
    "WLD/USDT": "worldcoin-wld",
    "TRUMP/USDT": "official-trump",
    "MNT/USDT": "mantle",
}


def fetch_historical_prices(
    token_pair: str,
    hours_back: int = 24,
    interval: str = "hourly"
) -> Optional[Dict]:
    """
    Fetch historical price data from CoinGecko.

    Args:
        token_pair: Trading pair (e.g., "BTC/USDT")
        hours_back: How many hours of history to fetch
        interval: "hourly" or "daily"

    Returns:
        Dict with prices at different times, or None on error
    """
    coin_id = PAIR_TO_COINGECKO.get(token_pair)
    if not coin_id:
        print(f"[ERROR] Unknown token pair: {token_pair}")
        return None

    # Calculate days for API (CoinGecko uses days, not hours)
    days = max(1, hours_back // 24 + 1)

    try:
        url = f"https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
        params = {
            "vs_currency": "usd",
            "days": days,
        }

        headers = {}
        if COINGECKO_API_KEY:
            headers["x-cg-demo-api-key"] = COINGECKO_API_KEY

        resp = requests.get(url, params=params, headers=headers, timeout=15)

        if resp.status_code == 429:
            print("[WARN] Rate limited by CoinGecko")
            return None

        resp.raise_for_status()
        data = resp.json()

        prices = data.get("prices", [])
        if not prices:
            print(f"[WARN] No price data returned for {token_pair}")
            return None

        # Process prices into hourly buckets
        hourly_prices = {}
        for ts_ms, price in prices:
            dt = datetime.fromtimestamp(ts_ms / 1000, tz=timezone.utc)
            hour_key = dt.strftime("%Y-%m-%d %H:00 UTC")
            hourly_prices[hour_key] = price

        # Get current price
        current_price = prices[-1][1] if prices else 0

        # Calculate stats
        all_prices = [p for _, p in prices]
        high_price = max(all_prices) if all_prices else 0
        low_price = min(all_prices) if all_prices else 0
        avg_price = sum(all_prices) / len(all_prices) if all_prices else 0

        # Get prices at specific times
        now = datetime.now(timezone.utc)
        result = {
            "token_pair": token_pair,
            "current_price": current_price,
            "high_24h": high_price,
            "low_24h": low_price,
            "avg_24h": avg_price,
            "hourly_prices": hourly_prices,
            "price_change_pct": ((current_price - all_prices[0]) / all_prices[0] * 100) if all_prices else 0,
            "fetched_at": now.strftime("%Y-%m-%d %H:%M:%S UTC"),
        }

        return result

    except requests.HTTPError as e:
        print(f"[ERROR] HTTP error fetching {token_pair}: {e}")
        return None
    except Exception as e:
        print(f"[ERROR] Error fetching {token_pair}: {e}")
        return None


def fetch_price_at_time(
    token_pair: str,
    target_time: datetime
) -> Optional[float]:
    """
    Get the approximate price at a specific time.

    Args:
        token_pair: Trading pair
        target_time: Target datetime (UTC)

    Returns:
        Price at that time, or None
    """
    # Calculate how many hours back
    now = datetime.now(timezone.utc)
    hours_back = int((now - target_time).total_seconds() / 3600) + 2

    data = fetch_historical_prices(token_pair, hours_back=hours_back)
    if not data:
        return None

    # Find closest hour
    target_key = target_time.strftime("%Y-%m-%d %H:00 UTC")

    if target_key in data["hourly_prices"]:
        return data["hourly_prices"][target_key]

    # Find closest available
    closest_price = None
    min_diff = float('inf')

    for hour_str, price in data["hourly_prices"].items():
        try:
            hour_dt = datetime.strptime(hour_str, "%Y-%m-%d %H:00 UTC").replace(tzinfo=timezone.utc)
            diff = abs((hour_dt - target_time).total_seconds())
            if diff < min_diff:
                min_diff = diff
                closest_price = price
        except:
            pass

    return closest_price


def analyze_price_by_hour(token_pair: str, days: int = 7) -> Dict:
    """
    Analyze price movements by hour of day over multiple days.
    Shows which hours tend to have price increases vs decreases.
    """
    coin_id = PAIR_TO_COINGECKO.get(token_pair)
    if not coin_id:
        return {"error": f"Unknown token pair: {token_pair}"}

    try:
        url = f"https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
        params = {
            "vs_currency": "usd",
            "days": days,
        }

        headers = {}
        if COINGECKO_API_KEY:
            headers["x-cg-demo-api-key"] = COINGECKO_API_KEY

        resp = requests.get(url, params=params, headers=headers, timeout=15)
        resp.raise_for_status()
        data = resp.json()

        prices = data.get("prices", [])
        if len(prices) < 24:
            return {"error": "Not enough price data"}

        # Group prices by hour of day
        hour_changes: Dict[int, List[float]] = {h: [] for h in range(24)}

        for i in range(1, len(prices)):
            ts_ms = prices[i][0]
            price = prices[i][1]
            prev_price = prices[i-1][1]

            dt = datetime.fromtimestamp(ts_ms / 1000, tz=timezone.utc)
            hour = dt.hour

            if prev_price > 0:
                change_pct = (price - prev_price) / prev_price * 100
                hour_changes[hour].append(change_pct)

        # Calculate stats for each hour
        hour_stats = {}
        for hour in range(24):
            changes = hour_changes[hour]
            if changes:
                avg_change = sum(changes) / len(changes)
                positive = sum(1 for c in changes if c > 0)
                negative = sum(1 for c in changes if c < 0)
                hour_stats[hour] = {
                    "avg_change_pct": avg_change,
                    "positive_moves": positive,
                    "negative_moves": negative,
                    "bullish_bias": positive > negative,
                    "total_samples": len(changes),
                }

        # Find best/worst hours
        best_hours = sorted(hour_stats.items(), key=lambda x: x[1]["avg_change_pct"], reverse=True)[:3]
        worst_hours = sorted(hour_stats.items(), key=lambda x: x[1]["avg_change_pct"])[:3]

        return {
            "token_pair": token_pair,
            "analysis_days": days,
            "hour_stats": hour_stats,
            "best_hours_for_long": [
                {"hour": h, "avg_change": s["avg_change_pct"], "bullish_pct": s["positive_moves"] / s["total_samples"] * 100}
                for h, s in best_hours
            ],
            "best_hours_for_short": [
                {"hour": h, "avg_change": s["avg_change_pct"], "bearish_pct": s["negative_moves"] / s["total_samples"] * 100}
                for h, s in worst_hours
            ],
        }

    except Exception as e:
        return {"error": str(e)}


def fetch_signal_history(token_pair: str, limit: int = 20) -> Dict:
    """Fetch signal history from verifier API."""
    try:
        url = f"{VERIFIER_API_URL}/api/v1/signals/history"
        params = {"token_pair": token_pair, "limit": limit}
        resp = requests.get(url, params=params, timeout=10)

        if resp.status_code == 200:
            return resp.json()
        elif resp.status_code == 404:
            return {"signals": [], "message": "No history found"}
        else:
            return {"error": f"HTTP {resp.status_code}"}
    except Exception as e:
        return {"error": str(e)}


# ─── TESTS ───────────────────────────────────────────────────────────────────

def test_historical_prices():
    """Test historical price fetching."""
    print("\n" + "="*60)
    print("TEST: Historical Price Fetching")
    print("="*60)

    test_pairs = ["BTC/USDT", "ETH/USDT", "SOL/USDT", "DOGE/USDT"]

    for pair in test_pairs:
        print(f"\n[{pair}]")
        data = fetch_historical_prices(pair, hours_back=24)

        if data:
            print(f"  Current Price: ${data['current_price']:.4f}")
            print(f"  24h High: ${data['high_24h']:.4f}")
            print(f"  24h Low: ${data['low_24h']:.4f}")
            print(f"  24h Change: {data['price_change_pct']:.2f}%")
            print(f"  Hourly data points: {len(data['hourly_prices'])}")

            # Show last 6 hours
            hours = list(data['hourly_prices'].items())[-6:]
            print("  Last 6 hours:")
            for hour, price in hours:
                print(f"    {hour}: ${price:.4f}")
        else:
            print("  FAILED to fetch data")

        time.sleep(1)  # Rate limit


def test_price_at_time():
    """Test fetching price at specific time."""
    print("\n" + "="*60)
    print("TEST: Price at Specific Time")
    print("="*60)

    pair = "BTC/USDT"

    # Test: yesterday at 2pm UTC
    yesterday_2pm = datetime.now(timezone.utc).replace(hour=14, minute=0, second=0, microsecond=0) - timedelta(days=1)

    print(f"\n[{pair}]")
    print(f"  Target time: {yesterday_2pm.strftime('%Y-%m-%d %H:%M UTC')}")

    price = fetch_price_at_time(pair, yesterday_2pm)
    if price:
        print(f"  Price at that time: ${price:.2f}")
    else:
        print("  FAILED to fetch price")

    # Test: 6 hours ago
    six_hours_ago = datetime.now(timezone.utc) - timedelta(hours=6)
    print(f"\n  Target time: {six_hours_ago.strftime('%Y-%m-%d %H:%M UTC')} (6h ago)")

    price = fetch_price_at_time(pair, six_hours_ago)
    if price:
        print(f"  Price at that time: ${price:.2f}")
    else:
        print("  FAILED to fetch price")


def test_hour_analysis():
    """Test hourly pattern analysis."""
    print("\n" + "="*60)
    print("TEST: Hourly Pattern Analysis (7 days)")
    print("="*60)

    pair = "BTC/USDT"
    print(f"\n[{pair}]")

    analysis = analyze_price_by_hour(pair, days=7)

    if "error" in analysis:
        print(f"  ERROR: {analysis['error']}")
        return

    print("\n  Best hours for LONG (most bullish):")
    for item in analysis["best_hours_for_long"]:
        print(f"    {item['hour']:02d}:00 UTC - avg +{item['avg_change']:.3f}% ({item['bullish_pct']:.0f}% bullish)")

    print("\n  Best hours for SHORT (most bearish):")
    for item in analysis["best_hours_for_short"]:
        print(f"    {item['hour']:02d}:00 UTC - avg {item['avg_change']:.3f}% ({item['bearish_pct']:.0f}% bearish)")


def test_signal_history():
    """Test signal history fetching from verifier."""
    print("\n" + "="*60)
    print("TEST: Signal History from Verifier")
    print("="*60)

    pair = "BTC/USDT"
    print(f"\n[{pair}]")

    history = fetch_signal_history(pair)

    if "error" in history:
        print(f"  ERROR: {history['error']}")
    elif not history.get("signals"):
        print("  No signal history found (this is expected for new deployments)")
    else:
        signals = history["signals"]
        print(f"  Found {len(signals)} signals")

        wins = sum(1 for s in signals if s.get("state") == "WIN")
        losses = sum(1 for s in signals if s.get("state") == "LOSS")
        print(f"  Wins: {wins}, Losses: {losses}")


if __name__ == "__main__":
    print("="*60)
    print("DATA FETCHING TEST SUITE")
    print("="*60)
    print(f"CoinGecko API Key: {'SET' if COINGECKO_API_KEY else 'NOT SET'}")
    print(f"Verifier API: {VERIFIER_API_URL}")

    test_historical_prices()
    test_price_at_time()
    test_hour_analysis()
    test_signal_history()

    print("\n" + "="*60)
    print("TESTS COMPLETE")
    print("="*60)
