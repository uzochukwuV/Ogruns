"""
Signal Generator with Weight-Based Scoring
Generates trading signals based on momentum, volume, and reversal analysis
"""
import math
import time
from typing import Dict, Literal, Optional, Tuple
from dataclasses import dataclass


@dataclass
class SignalPayload:
    """Signal payload matching the Verifier API format"""
    token_pair: str
    exchange: str
    direction: Literal["long", "short"]
    entry_price: float
    take_profit: float
    stop_loss: float
    expiry_time: int
    weight_pct: int
    trade_type: Literal["spot", "perpetual", "futures"] = "spot"
    leverage: int = 1


def compute_reversal_score(coin_data: Dict) -> Dict:
    """
    Compute a reversal probability score (0-100) based on momentum indicators.
    Higher score = higher probability of reversal.
    """
    h1 = coin_data.get("change_1h", 0)
    h24 = coin_data.get("change_24h", 0)
    d7 = coin_data.get("change_7d", 0)
    vol = coin_data.get("volume_24h", 0)
    mcap = coin_data.get("market_cap", 1)
    price = coin_data.get("price", 0)
    ath = coin_data.get("ath", price) or price or 1
    high_24h = coin_data.get("high_24h", price)
    low_24h = coin_data.get("low_24h", price)

    direction = "BULL" if h24 >= 0 else "BEAR"

    # Momentum decay: is the 1h pace slowing vs 24h average hourly pace?
    avg_hourly = h24 / 24 if h24 != 0 else 0
    momentum_decay = (
        (h1 < avg_hourly) if direction == "BULL"
        else (h1 > avg_hourly)
    )

    # Volume spike: daily vol > 15% of market cap
    vol_ratio = vol / mcap if mcap > 0 else 0
    vol_spike = vol_ratio > 0.15

    # Timeframe divergence: not all 1h/24h/7d pointing same way
    signs = [math.copysign(1, x) for x in [h1, h24, d7] if x != 0]
    all_aligned = len(set(signs)) == 1 if signs else False

    # Overextension: > 7% move in 24h
    overextended = abs(h24) > 7

    # ATH proximity (for bull moves - near ATH means less reversal room)
    ath_pct = ((price - ath) / ath * 100) if ath > 0 else 0

    # Candlestick pattern analysis
    pattern_analysis = coin_data.get("_pattern_analysis", {})
    bullish_bias = pattern_analysis.get("bullish_bias", 0)
    bearish_bias = pattern_analysis.get("bearish_bias", 0)
    patterns = pattern_analysis.get("patterns", [])

    # Calculate score
    score = 50
    if momentum_decay:
        score += 20
    if vol_spike:
        score += 15
    if not all_aligned:
        score += 15
    if overextended:
        score += 10
    if direction == "BULL" and ath_pct > -10:
        score -= 10

    # Incorporate candlestick patterns
    # For BULL trend, bearish patterns increase reversal probability
    # For BEAR trend, bullish patterns increase reversal probability
    if direction == "BULL":
        score += bearish_bias
    else:
        score += bullish_bias

    score = max(0, min(100, score))

    return {
        "score": score,
        "direction": direction,
        "reversal_dir": "BEAR REVERSAL" if direction == "BULL" else "BULL REVERSAL",
        "momentum_decay": momentum_decay,
        "vol_spike": vol_spike,
        "all_aligned": all_aligned,
        "overextended": overextended,
        "vol_ratio": round(vol_ratio * 100, 1),
        "patterns": patterns,
        "bullish_bias": bullish_bias,
        "bearish_bias": bearish_bias,
    }


def compute_delayed_score(coin_data: Dict, pattern_analysis: Dict) -> Tuple[int, bool]:
    """
    Compute score for delayed analysis tokens.
    Returns (score, should_signal) - should_signal is True if confident enough.
    """
    h24 = coin_data.get("change_24h", 0)
    direction = "BULL" if h24 >= 0 else "BEAR"

    bullish_bias = pattern_analysis.get("bullish_bias", 0)
    bearish_bias = pattern_analysis.get("bearish_bias", 0)

    base_score = 50
    if direction == "BULL":
        base_score += bearish_bias
    else:
        base_score += bullish_bias

    base_score = max(0, min(100, base_score))

    should_signal = base_score >= 65

    return base_score, should_signal


def generate_signal(coin_data: Dict, min_score: int = 55) -> Optional[SignalPayload]:
    """
    Generate a trading signal for a coin if it meets the reversal score threshold.

    Args:
        coin_data: Coin data from scanner.get_coin_data()
        min_score: Minimum reversal score to generate a signal (default: 55)

    Returns:
        SignalPayload if signal qualifies, None otherwise
    """
    analysis = compute_reversal_score(coin_data)

    if analysis["score"] < min_score:
        return None

    price = coin_data["price"]
    if price <= 0:
        return None

    # Determine signal direction (opposite of current trend for reversal)
    # If BULL trend with high reversal score -> expect BEAR reversal -> go SHORT
    # If BEAR trend with high reversal score -> expect BULL reversal -> go LONG
    direction: Literal["long", "short"] = "short" if analysis["direction"] == "BULL" else "long"

    # Calculate ATR for TP/SL (use 24h range or fallback to price change)
    high_24h = coin_data.get("high_24h") or 0
    low_24h = coin_data.get("low_24h") or 0

    if low_24h > 0 and high_24h > low_24h:
        atr_pct = ((high_24h - low_24h) / low_24h) * 100
    else:
        # Fallback: use 24h price change as volatility estimate
        raw_change = abs(coin_data.get("change_24h", 0))
        atr_pct = max(raw_change, 3)  # Minimum 3% ATR

    # Ensure minimum ATR of 2% for meaningful TP/SL
    atr_pct = max(atr_pct, 2.0)

    # Risk/Reward: 1:2 ratio (risk half of ATR, reward is full ATR)
    risk_pct = max(atr_pct * 0.5, 1.0)  # Minimum 1% risk
    reward_pct = max(risk_pct * 2, 2.0)  # Minimum 2% reward

    if direction == "long":
        take_profit = price * (1 + reward_pct / 100)
        stop_loss = price * (1 - risk_pct / 100)
    else:
        take_profit = price * (1 - reward_pct / 100)
        stop_loss = price * (1 + risk_pct / 100)

    # Weight (confidence) = reversal score
    weight_pct = analysis["score"]

    # Expiry: Dynamic based on volatility
    # Higher volatility → longer expiry (price needs more time to move)
    # Base: 30 min, add 10 min per 1% ATR above 2%, cap at 120 min (2 hours)
    base_minutes = 30
    extra_minutes = int((atr_pct - 2) * 10)  # 10 min per 1% ATR above 2%
    expiry_minutes = min(base_minutes + extra_minutes, 120)  # Cap at 2 hours
    expiry_time = int(time.time()) + expiry_minutes * 60

    return SignalPayload(
        token_pair=coin_data["token_pair"],
        exchange="coingecko",
        direction=direction,
        entry_price=round(price, 8),
        take_profit=round(take_profit, 8),
        stop_loss=round(stop_loss, 8),
        expiry_time=expiry_time,
        weight_pct=weight_pct,
        trade_type="spot",
        leverage=1,
    )


if __name__ == "__main__":
    # Test with mock data
    mock_coin = {
        "token_pair": "BTC/USDT",
        "price": 65000.0,
        "change_1h": -0.5,
        "change_24h": 5.5,
        "change_7d": -2.0,
        "volume_24h": 30_000_000_000,
        "market_cap": 1_200_000_000_000,
        "high_24h": 66000.0,
        "low_24h": 62000.0,
        "ath": 69000.0,
    }

    analysis = compute_reversal_score(mock_coin)
    print(f"Analysis: {analysis}")

    signal = generate_signal(mock_coin, min_score=50)
    if signal:
        print(f"\nSignal Generated:")
        print(f"  {signal.direction.upper()} {signal.token_pair}")
        print(f"  Entry: ${signal.entry_price:.2f}")
        print(f"  TP: ${signal.take_profit:.2f}")
        print(f"  SL: ${signal.stop_loss:.2f}")
        print(f"  Weight: {signal.weight_pct}%")
    else:
        print("No signal generated (score too low)")
