"""
AI Signal Analyzer using 0G Compute
Implements ReAct-style tool calling for intelligent signal validation.
"""
import json
import os
import re
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Dict, List, Literal, Optional, Tuple

import requests
from dotenv import load_dotenv

load_dotenv()


@dataclass
class AnalysisResult:
    """Result of AI signal analysis"""
    approved: bool
    confidence: float  # 0-100
    reasoning: str
    time_analysis: Optional[str] = None
    historical_win_rate: Optional[float] = None
    recommendation: Optional[str] = None


class SignalAnalyzer:
    """
    AI-powered signal analyzer using 0G Compute.
    Uses ReAct-style prompting for iterative tool-based analysis.
    """

    def __init__(self):
        self.api_key = os.getenv("ZG_AI_API_KEY", "")
        self.api_url = os.getenv(
            "ZG_AI_API_URL",
            "https://router-api-testnet.integratenetwork.work/v1/chat/completions"
        )
        self.model = os.getenv("ZG_AI_MODEL", "qwen/qwen-2.5-7b-instruct")
        self.verifier_api = os.getenv("VERIFIER_API_URL", "http://localhost:8080")
        self.enabled = bool(self.api_key)
        self.max_iterations = 5

        # Cache for historical data
        self._history_cache: Dict[str, List[dict]] = {}
        self._cache_ttl = 300  # 5 minutes
        self._cache_time: Dict[str, float] = {}

        if self.enabled:
            print(f"[AI] Analyzer initialized with model: {self.model}")
        else:
            print("[AI] Analyzer disabled (ZG_AI_API_KEY not set)")

    def is_enabled(self) -> bool:
        return self.enabled

    # ─── TOOLS ───────────────────────────────────────────────────────────────

    def tool_get_signal_history(self, token_pair: str, limit: int = 20) -> str:
        """Fetch historical signals for a token pair from the verifier API."""
        cache_key = f"history_{token_pair}"

        # Check cache
        if cache_key in self._history_cache:
            if time.time() - self._cache_time.get(cache_key, 0) < self._cache_ttl:
                history = self._history_cache[cache_key]
                return self._format_history(history, token_pair)

        try:
            url = f"{self.verifier_api}/api/v1/signals/history"
            params = {"token_pair": token_pair, "limit": limit}
            resp = requests.get(url, params=params, timeout=10)

            if resp.status_code == 200:
                data = resp.json()
                history = data.get("signals", [])
                self._history_cache[cache_key] = history
                self._cache_time[cache_key] = time.time()
                return self._format_history(history, token_pair)
            else:
                return f"No historical data found for {token_pair}"
        except Exception as e:
            return f"Error fetching history: {str(e)}"

    def _format_history(self, history: List[dict], token_pair: str) -> str:
        """Format signal history for AI consumption."""
        if not history:
            return f"No historical signals found for {token_pair}"

        wins = sum(1 for s in history if s.get("state") == "WIN")
        losses = sum(1 for s in history if s.get("state") == "LOSS")
        total = len(history)
        win_rate = (wins / total * 100) if total > 0 else 0

        # Analyze by direction
        long_signals = [s for s in history if s.get("envelope", {}).get("payload", {}).get("direction", "").lower() == "long"]
        short_signals = [s for s in history if s.get("envelope", {}).get("payload", {}).get("direction", "").lower() == "short"]

        long_wins = sum(1 for s in long_signals if s.get("state") == "WIN")
        short_wins = sum(1 for s in short_signals if s.get("state") == "WIN")

        result = f"""
SIGNAL HISTORY FOR {token_pair}:
- Total signals: {total}
- Wins: {wins}, Losses: {losses}
- Overall win rate: {win_rate:.1f}%
- LONG signals: {len(long_signals)} (wins: {long_wins}, rate: {long_wins/len(long_signals)*100 if long_signals else 0:.1f}%)
- SHORT signals: {len(short_signals)} (wins: {short_wins}, rate: {short_wins/len(short_signals)*100 if short_signals else 0:.1f}%)
"""
        return result.strip()

    def tool_get_time_analysis(self, token_pair: str) -> str:
        """Analyze signal performance by time of day."""
        cache_key = f"history_{token_pair}"
        history = self._history_cache.get(cache_key, [])

        if not history:
            # Try to fetch
            self.tool_get_signal_history(token_pair, limit=50)
            history = self._history_cache.get(cache_key, [])

        if not history:
            return f"No historical data for time analysis of {token_pair}"

        # Analyze by hour
        hour_stats: Dict[int, Dict[str, int]] = {}
        for signal in history:
            ts = signal.get("envelope", {}).get("timestamp", 0)
            if ts:
                hour = datetime.fromtimestamp(ts, tz=timezone.utc).hour
                if hour not in hour_stats:
                    hour_stats[hour] = {"wins": 0, "losses": 0}
                if signal.get("state") == "WIN":
                    hour_stats[hour]["wins"] += 1
                elif signal.get("state") == "LOSS":
                    hour_stats[hour]["losses"] += 1

        # Find best/worst hours
        best_hours = []
        worst_hours = []

        for hour, stats in sorted(hour_stats.items()):
            total = stats["wins"] + stats["losses"]
            if total >= 2:  # Need at least 2 signals for significance
                win_rate = stats["wins"] / total * 100
                if win_rate >= 70:
                    best_hours.append((hour, win_rate, total))
                elif win_rate <= 30:
                    worst_hours.append((hour, win_rate, total))

        result = f"""
TIME ANALYSIS FOR {token_pair}:
Current UTC hour: {datetime.now(timezone.utc).hour}

Best performing hours (>70% win rate):
{self._format_hours(best_hours) if best_hours else "  No clear best hours identified"}

Worst performing hours (<30% win rate):
{self._format_hours(worst_hours) if worst_hours else "  No clear worst hours identified"}

Hour breakdown:
"""
        for hour in sorted(hour_stats.keys()):
            stats = hour_stats[hour]
            total = stats["wins"] + stats["losses"]
            win_rate = stats["wins"] / total * 100 if total > 0 else 0
            result += f"  {hour:02d}:00 UTC - {stats['wins']}W/{stats['losses']}L ({win_rate:.0f}%)\n"

        return result.strip()

    def _format_hours(self, hours: List[Tuple[int, float, int]]) -> str:
        if not hours:
            return "  None"
        return "\n".join([f"  {h:02d}:00 UTC - {wr:.0f}% win rate ({t} signals)" for h, wr, t in hours])

    def tool_get_current_time(self) -> str:
        """Get current time information."""
        now = datetime.now(timezone.utc)

        # Determine market session
        hour = now.hour
        if 0 <= hour < 8:
            session = "Asian session (Tokyo/Sydney)"
        elif 8 <= hour < 14:
            session = "European session (London)"
        elif 14 <= hour < 21:
            session = "US session (New York)"
        else:
            session = "Late US / Early Asian transition"

        day_name = now.strftime("%A")
        is_weekend = day_name in ["Saturday", "Sunday"]

        return f"""
CURRENT TIME INFO:
- UTC Time: {now.strftime("%Y-%m-%d %H:%M:%S")}
- Day: {day_name}
- Hour (UTC): {hour}
- Market Session: {session}
- Weekend: {"Yes (lower volume expected)" if is_weekend else "No"}
"""

    def tool_get_recent_price_action(self, token_pair: str) -> str:
        """Get recent price action for the token (from CoinGecko)."""
        coin_id = self._get_coingecko_id(token_pair)
        if not coin_id:
            return f"Price data not available for {token_pair}"

        try:
            url = "https://api.coingecko.com/api/v3/coins/markets"
            params = {
                "vs_currency": "usd",
                "ids": coin_id,
                "price_change_percentage": "1h,24h,7d"
            }

            resp = requests.get(url, params=params, timeout=10)
            if resp.status_code != 200:
                return f"Failed to fetch price data for {token_pair}"

            data = resp.json()
            if not data:
                return f"No price data found for {token_pair}"

            coin = data[0]
            return f"""
RECENT PRICE ACTION FOR {token_pair}:
- Current Price: ${coin.get('current_price', 0):.8f}
- 1h Change: {coin.get('price_change_percentage_1h_in_currency', 0):.2f}%
- 24h Change: {coin.get('price_change_percentage_24h_in_currency', 0):.2f}%
- 7d Change: {coin.get('price_change_percentage_7d_in_currency', 0):.2f}%
- 24h High: ${coin.get('high_24h', 0):.8f}
- 24h Low: ${coin.get('low_24h', 0):.8f}
- 24h Volume: ${coin.get('total_volume', 0):,.0f}
"""
        except Exception as e:
            return f"Error fetching price data: {str(e)}"

    def tool_get_hourly_patterns(self, token_pair: str) -> str:
        """Analyze price patterns by hour of day over 7 days."""
        coin_id = self._get_coingecko_id(token_pair)
        if not coin_id:
            return f"Price pattern data not available for {token_pair}"

        try:
            url = f"https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
            params = {"vs_currency": "usd", "days": 7}

            resp = requests.get(url, params=params, timeout=15)
            if resp.status_code != 200:
                return f"Failed to fetch pattern data for {token_pair}"

            data = resp.json()
            prices = data.get("prices", [])
            if len(prices) < 48:
                return f"Not enough price data for {token_pair}"

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

            # Find best/worst hours
            hour_stats = []
            for hour in range(24):
                changes = hour_changes[hour]
                if changes:
                    avg_change = sum(changes) / len(changes)
                    positive = sum(1 for c in changes if c > 0)
                    total = len(changes)
                    hour_stats.append({
                        "hour": hour,
                        "avg_change": avg_change,
                        "bullish_pct": positive / total * 100,
                        "bearish_pct": (total - positive) / total * 100,
                    })

            # Sort by avg change
            hour_stats.sort(key=lambda x: x["avg_change"], reverse=True)
            best_for_long = hour_stats[:3]
            best_for_short = hour_stats[-3:]

            current_hour = datetime.now(timezone.utc).hour
            current_stat = next((h for h in hour_stats if h["hour"] == current_hour), None)

            result = f"""
HOURLY PRICE PATTERNS FOR {token_pair} (7-day analysis):

Current hour: {current_hour:02d}:00 UTC
"""
            if current_stat:
                result += f"Current hour stats: avg {current_stat['avg_change']:+.3f}%, {current_stat['bullish_pct']:.0f}% bullish\n"

            result += "\nBest hours for LONG (historically bullish):\n"
            for h in best_for_long:
                result += f"  {h['hour']:02d}:00 UTC - avg {h['avg_change']:+.3f}% ({h['bullish_pct']:.0f}% bullish)\n"

            result += "\nBest hours for SHORT (historically bearish):\n"
            for h in best_for_short:
                result += f"  {h['hour']:02d}:00 UTC - avg {h['avg_change']:+.3f}% ({h['bearish_pct']:.0f}% bearish)\n"

            # Check if current hour aligns with direction
            if current_stat:
                if current_stat["avg_change"] > 0.05:
                    result += f"\n[!] Current hour ({current_hour:02d}:00) is historically BULLISH - SHORT signals may underperform"
                elif current_stat["avg_change"] < -0.05:
                    result += f"\n[!] Current hour ({current_hour:02d}:00) is historically BEARISH - LONG signals may underperform"
                else:
                    result += f"\n[OK] Current hour ({current_hour:02d}:00) is neutral"

            return result.strip()

        except Exception as e:
            return f"Error analyzing patterns: {str(e)}"

    def tool_get_volume_analysis(self, token_pair: str) -> str:
        """Analyze trading volume patterns."""
        coin_id = self._get_coingecko_id(token_pair)
        if not coin_id:
            return f"Volume data not available for {token_pair}"

        try:
            # Get 7-day volume data
            url = f"https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
            params = {"vs_currency": "usd", "days": 7}

            resp = requests.get(url, params=params, timeout=15)
            if resp.status_code != 200:
                return f"Failed to fetch volume data for {token_pair}"

            data = resp.json()
            volumes = data.get("total_volumes", [])
            if len(volumes) < 24:
                return f"Not enough volume data for {token_pair}"

            # Calculate volume stats
            recent_volumes = [v[1] for v in volumes[-24:]]  # Last 24 data points
            older_volumes = [v[1] for v in volumes[:-24]] if len(volumes) > 24 else recent_volumes

            avg_recent = sum(recent_volumes) / len(recent_volumes)
            avg_older = sum(older_volumes) / len(older_volumes) if older_volumes else avg_recent
            current_vol = recent_volumes[-1] if recent_volumes else 0

            # Volume change
            vol_change_pct = ((avg_recent - avg_older) / avg_older * 100) if avg_older > 0 else 0
            current_vs_avg = ((current_vol - avg_recent) / avg_recent * 100) if avg_recent > 0 else 0

            # Detect spike
            is_spike = current_vs_avg > 50  # 50% above average
            is_low = current_vs_avg < -30  # 30% below average

            result = f"""
VOLUME ANALYSIS FOR {token_pair}:

Current Volume: ${current_vol:,.0f}
24h Average: ${avg_recent:,.0f}
7-day Average: ${avg_older:,.0f}

Volume vs 24h Avg: {current_vs_avg:+.1f}%
Recent vs Older Trend: {vol_change_pct:+.1f}%
"""
            if is_spike:
                result += "\n[SPIKE] Unusual high volume detected - increased volatility expected"
            elif is_low:
                result += "\n[LOW] Below average volume - moves may lack conviction"
            else:
                result += "\n[NORMAL] Volume is within normal range"

            return result.strip()

        except Exception as e:
            return f"Error analyzing volume: {str(e)}"

    def tool_get_btc_correlation(self, token_pair: str) -> str:
        """Check BTC trend and correlation for altcoin signals."""
        if token_pair == "BTC/USDT":
            return "This is BTC - correlation check not applicable"

        try:
            # Get BTC current trend
            url = "https://api.coingecko.com/api/v3/coins/markets"
            params = {
                "vs_currency": "usd",
                "ids": "bitcoin",
                "price_change_percentage": "1h,24h"
            }

            resp = requests.get(url, params=params, timeout=10)
            if resp.status_code != 200:
                return "Failed to fetch BTC data"

            data = resp.json()
            if not data:
                return "No BTC data available"

            btc = data[0]
            btc_1h = btc.get('price_change_percentage_1h_in_currency', 0) or 0
            btc_24h = btc.get('price_change_percentage_24h_in_currency', 0) or 0

            # Determine BTC trend
            if btc_1h > 0.5 and btc_24h > 0:
                btc_trend = "BULLISH"
                btc_impact = "Altcoins tend to follow BTC up - LONG signals favored"
            elif btc_1h < -0.5 and btc_24h < 0:
                btc_trend = "BEARISH"
                btc_impact = "Altcoins tend to follow BTC down - SHORT signals favored"
            else:
                btc_trend = "NEUTRAL"
                btc_impact = "BTC is ranging - altcoin moves may be independent"

            result = f"""
BTC CORRELATION CHECK:

BTC 1h Change: {btc_1h:+.2f}%
BTC 24h Change: {btc_24h:+.2f}%
BTC Trend: {btc_trend}

Impact on {token_pair}: {btc_impact}
"""
            if btc_trend == "BEARISH":
                result += "\n[!] Warning: BTC is weak - LONG altcoin signals carry extra risk"
            elif btc_trend == "BULLISH":
                result += "\n[!] Warning: BTC is strong - SHORT altcoin signals carry extra risk"

            return result.strip()

        except Exception as e:
            return f"Error checking BTC correlation: {str(e)}"

    def tool_get_market_sentiment(self) -> str:
        """Get overall market sentiment (Fear & Greed Index)."""
        try:
            url = "https://api.alternative.me/fng/"
            resp = requests.get(url, timeout=10)

            if resp.status_code != 200:
                return "Failed to fetch market sentiment"

            data = resp.json()
            if not data.get("data"):
                return "No sentiment data available"

            current = data["data"][0]
            value = int(current.get("value", 50))
            classification = current.get("value_classification", "Neutral")

            # Historical comparison (if available)
            yesterday = data["data"][1] if len(data["data"]) > 1 else current
            yesterday_value = int(yesterday.get("value", value))
            change = value - yesterday_value

            result = f"""
MARKET SENTIMENT (Fear & Greed Index):

Current Value: {value}/100
Classification: {classification}
Change from Yesterday: {change:+d}

"""
            if value <= 25:
                result += "EXTREME FEAR - Market is very bearish, potential bounce opportunity\n"
                result += "-> LONG signals may find good entries at oversold levels"
            elif value <= 40:
                result += "FEAR - Market is cautious\n"
                result += "-> Trend-following SHORT signals have momentum"
            elif value <= 60:
                result += "NEUTRAL - No strong sentiment bias\n"
                result += "-> Both directions viable, focus on technicals"
            elif value <= 75:
                result += "GREED - Market is optimistic\n"
                result += "-> LONG signals have momentum, but watch for overextension"
            else:
                result += "EXTREME GREED - Market may be overheated\n"
                result += "-> SHORT signals may catch reversal, LONG signals risky"

            return result.strip()

        except Exception as e:
            return f"Error fetching sentiment: {str(e)}"

    def tool_get_volatility_analysis(self, token_pair: str) -> str:
        """Analyze price volatility (ATR-like metric)."""
        coin_id = self._get_coingecko_id(token_pair)
        if not coin_id:
            return f"Volatility data not available for {token_pair}"

        try:
            url = f"https://api.coingecko.com/api/v3/coins/{coin_id}/market_chart"
            params = {"vs_currency": "usd", "days": 7}

            resp = requests.get(url, params=params, timeout=15)
            if resp.status_code != 200:
                return f"Failed to fetch volatility data for {token_pair}"

            data = resp.json()
            prices = data.get("prices", [])
            if len(prices) < 48:
                return f"Not enough data for volatility analysis"

            # Calculate hourly ranges (pseudo-ATR)
            hourly_ranges = []
            for i in range(1, len(prices)):
                price = prices[i][1]
                prev_price = prices[i-1][1]
                range_pct = abs(price - prev_price) / prev_price * 100
                hourly_ranges.append(range_pct)

            # Recent vs older volatility
            recent_ranges = hourly_ranges[-24:]
            older_ranges = hourly_ranges[:-24] if len(hourly_ranges) > 24 else hourly_ranges

            avg_recent_vol = sum(recent_ranges) / len(recent_ranges)
            avg_older_vol = sum(older_ranges) / len(older_ranges) if older_ranges else avg_recent_vol
            max_recent = max(recent_ranges) if recent_ranges else 0

            vol_change = ((avg_recent_vol - avg_older_vol) / avg_older_vol * 100) if avg_older_vol > 0 else 0

            result = f"""
VOLATILITY ANALYSIS FOR {token_pair}:

Avg Hourly Move (24h): {avg_recent_vol:.3f}%
Avg Hourly Move (7d): {avg_older_vol:.3f}%
Max Hourly Move (24h): {max_recent:.3f}%
Volatility Trend: {vol_change:+.1f}%
"""
            if avg_recent_vol > 0.5:
                result += "\n[HIGH VOLATILITY] Large moves expected - wider SL recommended"
            elif avg_recent_vol < 0.1:
                result += "\n[LOW VOLATILITY] Tight range - smaller TP targets realistic"
            else:
                result += "\n[NORMAL VOLATILITY] Standard market conditions"

            if vol_change > 30:
                result += "\n[EXPANDING] Volatility increasing - breakout potential"
            elif vol_change < -30:
                result += "\n[CONTRACTING] Volatility decreasing - ranging market"

            return result.strip()

        except Exception as e:
            return f"Error analyzing volatility: {str(e)}"

    def _get_coingecko_id(self, token_pair: str) -> Optional[str]:
        """Get CoinGecko ID from token pair."""
        pair_to_id = {
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
        return pair_to_id.get(token_pair)

    # ─── TOOL EXECUTION ──────────────────────────────────────────────────────

    def _execute_tool(self, tool_name: str, args: str) -> str:
        """Execute a tool by name with given arguments."""
        tools = {
            "get_signal_history": lambda a: self.tool_get_signal_history(a.strip()),
            "get_time_analysis": lambda a: self.tool_get_time_analysis(a.strip()),
            "get_current_time": lambda _: self.tool_get_current_time(),
            "get_recent_price_action": lambda a: self.tool_get_recent_price_action(a.strip()),
            "get_hourly_patterns": lambda a: self.tool_get_hourly_patterns(a.strip()),
            "get_volume_analysis": lambda a: self.tool_get_volume_analysis(a.strip()),
            "get_volatility_analysis": lambda a: self.tool_get_volatility_analysis(a.strip()),
            "get_btc_correlation": lambda a: self.tool_get_btc_correlation(a.strip()),
            "get_market_sentiment": lambda _: self.tool_get_market_sentiment(),
        }

        if tool_name not in tools:
            return f"Unknown tool: {tool_name}"

        try:
            return tools[tool_name](args)
        except Exception as e:
            return f"Tool error: {str(e)}"

    def _parse_tool_call(self, content: str) -> Optional[Tuple[str, str]]:
        """Parse a tool call from AI response. Returns (tool_name, args) or None."""
        # Match patterns like:
        # TOOL: get_signal_history(BTC/USDT)
        # TOOL: get_time_analysis("ETH/USDT")
        pattern = r"TOOL:\s*(\w+)\s*\(\s*[\"']?([^)\"']*)[\"']?\s*\)"
        match = re.search(pattern, content, re.IGNORECASE)
        if match:
            return match.group(1), match.group(2)
        return None

    def _parse_final_decision(self, content: str) -> Optional[AnalysisResult]:
        """Parse final decision from AI response."""
        # Look for FINAL: followed by JSON
        pattern = r"FINAL:\s*(\{[^}]+\})"
        match = re.search(pattern, content, re.DOTALL)

        if match:
            try:
                data = json.loads(match.group(1))
                return AnalysisResult(
                    approved=data.get("approved", False),
                    confidence=data.get("confidence", 50),
                    reasoning=data.get("reasoning", "No reasoning provided"),
                    time_analysis=data.get("time_analysis"),
                    historical_win_rate=data.get("historical_win_rate"),
                    recommendation=data.get("recommendation")
                )
            except json.JSONDecodeError:
                pass

        # Fallback: look for simpler format
        if "APPROVE" in content.upper():
            conf_match = re.search(r"confidence[:\s]+(\d+)", content, re.IGNORECASE)
            confidence = float(conf_match.group(1)) if conf_match else 70
            return AnalysisResult(
                approved=True,
                confidence=confidence,
                reasoning=content[:200]
            )
        elif "REJECT" in content.upper():
            conf_match = re.search(r"confidence[:\s]+(\d+)", content, re.IGNORECASE)
            confidence = float(conf_match.group(1)) if conf_match else 70
            return AnalysisResult(
                approved=False,
                confidence=confidence,
                reasoning=content[:200]
            )

        return None

    # ─── MAIN ANALYSIS ───────────────────────────────────────────────────────

    def analyze_signal(
        self,
        token_pair: str,
        direction: str,
        entry_price: float,
        take_profit: float,
        stop_loss: float,
        weight_pct: float,
        min_confidence: float = 60.0
    ) -> AnalysisResult:
        """
        Analyze a trading signal using 0G Compute with iterative tool calling.

        Returns an AnalysisResult with approval decision and confidence.
        """
        if not self.enabled:
            # Fallback to heuristic analysis
            return self._heuristic_analysis(
                token_pair, direction, entry_price, take_profit, stop_loss, weight_pct
            )

        # Build initial context
        current_hour = datetime.now(timezone.utc).hour

        system_prompt = """You are a trading signal analyst AI. Your job is to analyze trading signals and decide whether to approve them based on historical price patterns, market context, and risk quality.

You have access to these tools:
- TOOL: get_hourly_patterns(TOKEN_PAIR) - IMPORTANT: Get 7-day hourly price patterns showing which hours are historically bullish/bearish
- TOOL: get_recent_price_action(TOKEN_PAIR) - Get current price and recent changes (1h, 24h, 7d)
- TOOL: get_volume_analysis(TOKEN_PAIR) - Check whether volume confirms the move or suggests weak conviction
- TOOL: get_volatility_analysis(TOKEN_PAIR) - Check current volatility, expansion/contraction, and whether TP/SL is realistic
- TOOL: get_btc_correlation(TOKEN_PAIR) - For altcoins, compare the signal direction against BTC market direction
- TOOL: get_market_sentiment() - Get overall crypto Fear & Greed market sentiment
- TOOL: get_signal_history(TOKEN_PAIR) - Get historical win/loss data from our platform (may be empty for new tokens)
- TOOL: get_current_time() - Get current UTC time and market session info

To use a tool, output: TOOL: tool_name(argument)
Then wait for the result.

ANALYSIS STRATEGY:
1. ALWAYS check get_hourly_patterns first - this is the most important data
2. Compare the signal direction (LONG/SHORT) with the current hour's historical bias
3. If the current hour is historically bearish and signal is SHORT, that's good alignment
4. If the current hour is historically bullish and signal is LONG, that's good alignment
5. Misalignment (e.g., SHORT signal during historically bullish hour) should reduce confidence
6. Check volume next: high volume confirms conviction; low volume reduces confidence
7. Check volatility to confirm TP/SL is achievable without excessive risk
8. For non-BTC tokens, check BTC correlation because BTC weakness hurts altcoin LONGs and BTC strength hurts altcoin SHORTs
9. Use market sentiment as a broad bias, not as a standalone reason to approve

When you have enough information to make a decision, output:
FINAL: {"approved": true/false, "confidence": 0-100, "reasoning": "your explanation", "recommendation": "optional advice"}

Be strict: Only approve signals with >60% confidence. Time alignment is important, but do not approve if volume, volatility, BTC trend, or sentiment strongly contradict the signal."""

        user_prompt = f"""Analyze this trading signal:

TOKEN: {token_pair}
DIRECTION: {direction.upper()}
ENTRY: ${entry_price}
TAKE PROFIT: ${take_profit}
STOP LOSS: ${stop_loss}
SIGNAL WEIGHT: {weight_pct}%
CURRENT HOUR (UTC): {current_hour}

IMPORTANT: Start by checking get_hourly_patterns, then use the non-time tools when needed to confirm or reject the signal."""

        messages = [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt}
        ]

        # ReAct loop
        for iteration in range(self.max_iterations):
            try:
                response = self._call_api(messages)

                if not response:
                    print(f"[AI] Empty response on iteration {iteration + 1}")
                    break

                # Check for tool call
                tool_call = self._parse_tool_call(response)
                if tool_call:
                    tool_name, args = tool_call
                    print(f"[AI] Calling tool: {tool_name}({args})")

                    tool_result = self._execute_tool(tool_name, args)

                    # Add assistant response and tool result to conversation
                    messages.append({"role": "assistant", "content": response})
                    messages.append({"role": "user", "content": f"TOOL RESULT:\n{tool_result}"})
                    continue

                # Check for final decision
                decision = self._parse_final_decision(response)
                if decision:
                    print(f"[AI] Decision: {'APPROVE' if decision.approved else 'REJECT'} (confidence: {decision.confidence}%)")
                    return decision

                # If neither, prompt for decision
                messages.append({"role": "assistant", "content": response})
                messages.append({"role": "user", "content": "Please make your final decision using the FINAL: format."})

            except Exception as e:
                print(f"[AI] Error on iteration {iteration + 1}: {e}")
                break

        # Fallback if no decision reached
        print("[AI] Max iterations reached, using heuristic fallback")
        return self._heuristic_analysis(
            token_pair, direction, entry_price, take_profit, stop_loss, weight_pct
        )

    def _call_api(self, messages: List[dict]) -> Optional[str]:
        """Call 0G Compute API."""
        try:
            payload = {
                "model": self.model,
                "messages": messages,
                "stream": False,
                "verify_tee": True
            }

            headers = {
                "Content-Type": "application/json",
                "Authorization": f"Bearer {self.api_key}"
            }

            resp = requests.post(
                self.api_url,
                json=payload,
                headers=headers,
                timeout=30
            )

            if resp.status_code != 200:
                print(f"[AI] API error: {resp.status_code} - {resp.text[:200]}")
                return None

            data = resp.json()
            choices = data.get("choices", [])
            if not choices:
                return None

            return choices[0].get("message", {}).get("content", "")

        except Exception as e:
            print(f"[AI] API call failed: {e}")
            return None

    def _heuristic_analysis(
        self,
        token_pair: str,
        direction: str,
        entry_price: float,
        take_profit: float,
        stop_loss: float,
        weight_pct: float
    ) -> AnalysisResult:
        """Fallback heuristic analysis when AI is unavailable."""
        confidence = 50.0
        reasons = []

        # Calculate R:R
        if direction.lower() == "long":
            tp_dist = (take_profit - entry_price) / entry_price * 100
            sl_dist = (entry_price - stop_loss) / entry_price * 100
        else:
            tp_dist = (entry_price - take_profit) / entry_price * 100
            sl_dist = (stop_loss - entry_price) / entry_price * 100

        rr_ratio = tp_dist / sl_dist if sl_dist > 0 else 0

        # R:R scoring
        if rr_ratio >= 2.0:
            confidence += 15
            reasons.append(f"Good R:R ratio ({rr_ratio:.1f}:1)")
        elif rr_ratio >= 1.5:
            confidence += 10
            reasons.append(f"Acceptable R:R ratio ({rr_ratio:.1f}:1)")
        elif rr_ratio < 1.0:
            confidence -= 15
            reasons.append(f"Poor R:R ratio ({rr_ratio:.1f}:1)")

        # Weight scoring
        if weight_pct >= 70:
            confidence += 10
            reasons.append(f"High signal weight ({weight_pct}%)")
        elif weight_pct < 50:
            confidence -= 10
            reasons.append(f"Low signal weight ({weight_pct}%)")

        # Time-based (simple heuristic)
        current_hour = datetime.now(timezone.utc).hour
        if 8 <= current_hour <= 16:  # European + early US session
            confidence += 5
            reasons.append("Active trading hours")

        approved = confidence >= 60

        return AnalysisResult(
            approved=approved,
            confidence=confidence,
            reasoning="; ".join(reasons) if reasons else "Heuristic analysis",
            recommendation="AI analysis unavailable, using basic heuristics"
        )


# ─── TEST ────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    analyzer = SignalAnalyzer()

    # Test with a sample signal
    result = analyzer.analyze_signal(
        token_pair="BTC/USDT",
        direction="short",
        entry_price=65000.0,
        take_profit=62000.0,
        stop_loss=66500.0,
        weight_pct=75
    )

    print(f"\n{'='*60}")
    print(f"ANALYSIS RESULT")
    print(f"{'='*60}")
    print(f"Approved: {result.approved}")
    print(f"Confidence: {result.confidence}%")
    print(f"Reasoning: {result.reasoning}")
    if result.recommendation:
        print(f"Recommendation: {result.recommendation}")
