"""
CoinGecko Market Scanner
Fetches /coins/markets and filters by price change percentage
"""
import requests
import time
from typing import List, Dict
from config import Config


# Token pair mapping: CoinGecko ID -> trading pair format
COINGECKO_TO_PAIR = {
    "bitcoin": "BTC/USDT",
    "ethereum": "ETH/USDT",
    "solana": "SOL/USDT",
    "binancecoin": "BNB/USDT",
    "ripple": "XRP/USDT",
    "cardano": "ADA/USDT",
    "dogecoin": "DOGE/USDT",
    "chainlink": "LINK/USDT",
    "avalanche-2": "AVAX/USDT",
    "polkadot": "DOT/USDT",
    "matic-network": "MATIC/USDT",
    "uniswap": "UNI/USDT",
    "litecoin": "LTC/USDT",
    "cosmos": "ATOM/USDT",
    "near": "NEAR/USDT",

    # Added from market list
    "bitcoin-cash": "BCH/USDT",
    "ethereum-classic": "ETC/USDT",
    "internet-computer": "ICP/USDT",
    "filecoin": "FIL/USDT",
    "render-token": "RENDER/USDT",
    "injective-protocol": "INJ/USDT",
    "aave": "AAVE/USDT",
    "arbitrum": "ARB/USDT",
    "aptos": "APT/USDT",
    "ondo-finance": "ONDO/USDT",
    "optimism": "OP/USDT",
    "sei-network": "SEI/USDT",
    "sui": "SUI/USDT",
    "toncoin": "TON/USDT",
    "pendle": "PENDLE/USDT",
    "jupiter-exchange-solana": "JUP/USDT",
    "celestia": "TIA/USDT",
    "kaspa": "KAS/USDT",
    "worldcoin-wld": "WLD/USDT",
    "pepe": "PEPE/USDT",
    "shiba-inu": "SHIB/USDT",
    "floki": "FLOKI/USDT",
    "bonk": "BONK/USDT",
    "dogwifcoin": "WIF/USDT",
    "tron": "TRX/USDT",
    "stellar": "XLM/USDT",
    "hedera-hashgraph": "HBAR/USDT",
    "algorand": "ALGO/USDT",
    "cosmos-hub": "ATOM/USDT",
    "crv": "CRV/USDT",
    "convex-finance": "CVX/USDT",
    "gmx": "GMX/USDT",
    "dydx-chain": "DYDX/USDT",
    "lido-dao": "LDO/USDT",
    "mantle": "MNT/USDT",
    "morpho": "MORPHO/USDT",
    "pancakeswap-token": "CAKE/USDT",
    "compound-governance-token": "COMP/USDT",
    "theta-token": "THETA/USDT",
    "zcash": "ZEC/USDT",
    "monero": "XMR/USDT",
    "dash": "DASH/USDT",
    "the-open-network": "TON/USDT",
    "bittensor": "TAO/USDT",
    "fetch-ai": "FET/USDT",
    "eigenlayer": "EIGEN/USDT",
    "layerzero": "ZRO/USDT",
    "aerodrome-finance": "AERO/USDT",
    "virtual-protocol": "VIRTUAL/USDT",
    "ondo": "ONDO/USDT",
    "zora": "ZORA/USDT",
    "story-protocol": "IP/USDT",
    "berachain": "BERA/USDT",
    "jito-governance-token": "JTO/USDT",
    "stacks": "STX/USDT",
    "polygon-ecosystem-token": "POL/USDT",
    "syrup": "SYRUP/USDT",
    "linea": "LINEA/USDT",
    "animecoin": "ANIME/USDT",
    "hyperliquid": "HYPE/USDT",
    "official-trump": "TRUMP/USDT",
    "fartcoin": "FARTCOIN/USDT",
    "pengu": "PENGU/USDT",
    "0g-labs": "0G/USDT",
}


def _request_with_retry(url: str, params: dict, headers: dict, timeout: int, max_retries: int = 3):
    """Make HTTP request with exponential backoff retry."""
    for attempt in range(max_retries):
        try:
            response = requests.get(url, params=params, headers=headers, timeout=timeout)
            response.raise_for_status()
            return response
        except (requests.exceptions.ConnectionError, requests.exceptions.SSLError) as e:
            if attempt < max_retries - 1:
                wait = (2 ** attempt) * 2
                print(f"Network error, retrying in {wait}s: {e.__class__.__name__}")
                time.sleep(wait)
            else:
                raise
        except requests.HTTPError:
            raise
    return None


def fetch_ohlc_data(coin_id: str, days: int = None, silent: bool = False) -> List[List]:
    """
    Fetch OHLC data from CoinGecko for candlestick pattern analysis.
    Returns list of [timestamp, open, high, low, close] arrays.
    """
    if days is None:
        days = Config.OHLC_DAYS

    url = f"{Config.COINGECKO_BASE_URL}/coins/{coin_id}/ohlc"
    params = {
        "vs_currency": "usd",
        "days": days,
    }

    headers = {}
    if Config.COINGECKO_API_KEY:
        headers["x-cg-demo-api-key"] = Config.COINGECKO_API_KEY

    try:
        response = _request_with_retry(url, params, headers, timeout=15)
        response.raise_for_status()
        ohlc = response.json()
        if not isinstance(ohlc, list):
            if not silent:
                print(f"OHLC fetch error for {coin_id}: unexpected response {ohlc}")
            return []
        return ohlc
    except requests.HTTPError as e:
        if not silent:
            print(f"OHLC fetch error for {coin_id}: HTTP {e.response.status_code}")
        return []
    except Exception as e:
        if not silent:
            print(f"OHLC fetch error for {coin_id}: {e}")
        return []


def detect_candlestick_patterns(ohlc_data: List[List]) -> Dict:
    """
    Detect bullish/bearish candlestick patterns from OHLC data.
    
    Patterns detected:
    - Hammer (bullish reversal)
    - Inverted Hammer (bearish reversal)
    - Bullish/Bearish Engulfing
    - Morning Star (bullish reversal)
    - Three Black Crows (bearish continuation)
    - Doji (indecision)
    - Shooting Star (bearish reversal)
    - Piercing Line (bullish reversal)
    - Dark Cloud Cover (bearish reversal)
    """
    if len(ohlc_data) < 3:
        return {"patterns": [], "bullish_bias": 0, "bearish_bias": 0, "ohlc_raw": ohlc_data}

    patterns = []
    bullish_bias = 0
    bearish_bias = 0

    recent = ohlc_data[-5:] if len(ohlc_data) >= 5 else ohlc_data

    def analyze_candle(candle):
        o, h, l, c = candle[1], candle[2], candle[3], candle[4]
        body = abs(c - o)
        wick_upper = h - max(o, c)
        wick_lower = min(o, c) - l
        total_range = h - l if h > l else 1
        return body, wick_upper, wick_lower, total_range

    # Single candle patterns
    for i, candle in enumerate(recent):
        body, wick_upper, wick_lower, total_range = analyze_candle(candle)

        # Hammer: small body, long lower wick
        if body > 0 and body / total_range < 0.3 and wick_lower / total_range > 0.6:
            patterns.append({"name": "hammer", "index": i, "bullish": True})
            bullish_bias += 15

        # Inverted Hammer: small body, long upper wick
        if body > 0 and body / total_range < 0.3 and wick_upper / total_range > 0.6:
            patterns.append({"name": "inverted_hammer", "index": i, "bullish": False})
            bearish_bias += 10

        # Shooting Star: bearish at uptrend top
        if body > 0 and body / total_range < 0.4 and wick_upper / total_range > 0.7:
            patterns.append({"name": "shooting_star", "index": i, "bullish": False})
            bearish_bias += 12

        # Doji: indecision (very small body)
        if body / total_range < 0.1:
            patterns.append({"name": "doji", "index": i, "bullish": None})

    # Two-candle patterns
    for i in range(1, len(recent)):
        prev = recent[i - 1]
        curr = recent[i]
        prev_body_val = prev[4] - prev[1]
        curr_body_val = curr[4] - curr[1]

        abs_prev = abs(prev_body_val)
        abs_curr = abs(curr_body_val)

        # Bullish engulfing
        if prev_body_val < 0 and curr_body_val > 0 and abs_curr > abs_prev:
            patterns.append({"name": "bullish_engulfing", "index": i, "bullish": True})
            bullish_bias += 20

        # Bearish engulfing
        if prev_body_val > 0 and curr_body_val < 0 and abs_curr > abs_prev:
            patterns.append({"name": "bearish_engulfing", "index": i, "bullish": False})
            bearish_bias += 20

        # Piercing Line
        if prev_body_val < 0 and curr_body_val > 0:
            body_mid = (prev[2] + prev[3]) / 2
            if curr[4] > body_mid and curr[4] > prev[1]:
                patterns.append({"name": "piercing_line", "index": i, "bullish": True})
                bullish_bias += 15

        # Dark Cloud Cover
        if prev_body_val > 0 and curr_body_val < 0:
            body_mid = (prev[2] + prev[3]) / 2
            if curr[4] < body_mid and curr[4] < prev[1]:
                patterns.append({"name": "dark_cloud_cover", "index": i, "bullish": False})
                bearish_bias += 15

    # Three-candle patterns
    if len(recent) >= 3:
        c1, c2, c3 = recent[-3], recent[-2], recent[-1]
        
        # Morning Star
        if c1[4] < c1[1] and c3[4] > c3[1]:
            if abs(c2[4] - c2[1]) < abs(c1[4] - c1[1]):
                patterns.append({"name": "morning_star", "index": 2, "bullish": True})
                bullish_bias += 25

        # Three Black Crows
        if all(c[4] < c[1] for c in [c1, c2, c3]):
            patterns.append({"name": "three_black_crows", "index": 2, "bullish": False})
            bearish_bias += 20

    return {
        "patterns": patterns,
        "bullish_bias": bullish_bias,
        "bearish_bias": bearish_bias,
        "net_bias": bullish_bias - bearish_bias,
        "ohlc_raw": ohlc_data,
    }


def fetch_market_movers(
    min_pct: float = 4.0,
    max_pct: float = 9.0,
    per_page: int = 250,
) -> List[Dict]:
    """
    Fetch coins from CoinGecko /coins/markets and filter by 24h price change.
    """
    url = f"{Config.COINGECKO_BASE_URL}/coins/markets"
    params = {
        "vs_currency": "usd",
        "order": "market_cap_desc",
        "per_page": per_page,
        "page": 1,
        "price_change_percentage": "1h,24h,7d",
        "sparkline": "false",
    }

    headers = {}
    if Config.COINGECKO_API_KEY:
        headers["x-cg-demo-api-key"] = Config.COINGECKO_API_KEY

    try:
        response = _request_with_retry(url, params, headers, timeout=30)
        response.raise_for_status()
        all_coins = response.json()
    except requests.HTTPError as e:
        raise e
    except Exception as e:
        raise e

    filtered = []
    for coin in all_coins:
        change_24h = coin.get("price_change_percentage_24h_in_currency") or 0
        abs_change = abs(change_24h)

        if coin["id"] in COINGECKO_TO_PAIR and min_pct <= abs_change <= max_pct:
            coin["_token_pair"] = COINGECKO_TO_PAIR[coin["id"]]
            filtered.append(coin)

    return filtered


def get_coin_data(coin: Dict) -> Dict:
    """Extract relevant data from a CoinGecko coin object."""
    return {
        "id": coin["id"],
        "symbol": coin["symbol"].upper(),
        "token_pair": coin.get("_token_pair", f"{coin['symbol'].upper()}/USDT"),
        "price": coin.get("current_price", 0),
        "change_1h": coin.get("price_change_percentage_1h_in_currency") or 0,
        "change_24h": coin.get("price_change_percentage_24h_in_currency") or 0,
        "change_7d": coin.get("price_change_percentage_7d_in_currency") or 0,
        "volume_24h": coin.get("total_volume") or 0,
        "market_cap": coin.get("market_cap") or 0,
        "high_24h": coin.get("high_24h") or 0,
        "low_24h": coin.get("low_24h") or 0,
        "ath": coin.get("ath") or 0,
        "atl": coin.get("atl") or 0,
    }


if __name__ == "__main__":
    print("Scanning for market movers (2-8% change)...")
    movers = fetch_market_movers(min_pct=2.0, max_pct=8.0)
    print(f"Found {len(movers)} coins")

    for coin in movers[:10]:
        data = get_coin_data(coin)
        print(f"  {data['symbol']}: ${data['price']:.2f} ({data['change_24h']:+.2f}%)")
