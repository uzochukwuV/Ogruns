"""
Configuration loader for the AI Trading Signal Agent
"""
import os
from dotenv import load_dotenv

load_dotenv()


class Config:
    # Agent identity
    AGENT_PRIVATE_KEY: str = os.getenv("AGENT_PRIVATE_KEY", "")

    # Verifier API
    VERIFIER_API_URL: str = os.getenv("VERIFIER_API_URL", "http://localhost:8080")

    # Scan parameters
    MIN_PRICE_CHANGE_PCT: float = float(os.getenv("MIN_PRICE_CHANGE_PCT", "2"))
    MAX_PRICE_CHANGE_PCT: float = float(os.getenv("MAX_PRICE_CHANGE_PCT", "8"))
    SCAN_INTERVAL_SEC: int = int(os.getenv("SCAN_INTERVAL_SEC", "90"))

    # Volume filter (default $5M minimum 24h volume)
    MIN_VOLUME_USD: float = float(os.getenv("MIN_VOLUME_USD", "5000000"))

    # Reversal-score thresholds
    MIN_SIGNAL_SCORE: int = int(os.getenv("MIN_SIGNAL_SCORE", "55"))
    WATCH_LOWER: int = int(os.getenv("WATCH_LOWER", "55"))
    WATCH_UPPER: int = int(os.getenv("WATCH_UPPER", "59"))
    MILD_LOWER: int = int(os.getenv("MILD_LOWER", "60"))
    MILD_UPPER: int = int(os.getenv("MILD_UPPER", "69"))
    MODERATE_LOWER: int = int(os.getenv("MODERATE_LOWER", "70"))
    MODERATE_UPPER: int = int(os.getenv("MODERATE_UPPER", "79"))
    STRONG_LOWER: int = int(os.getenv("STRONG_LOWER", "80"))

    # Delayed analysis delays (seconds)
    DELAYED_ANALYSIS_MILD_DELAY: int = int(os.getenv("DELAYED_ANALYSIS_MILD_DELAY", "900"))  # 15 min
    DELAYED_ANALYSIS_MODERATE_DELAY: int = int(os.getenv("DELAYED_ANALYSIS_MODERATE_DELAY", "1800"))  # 30 min

    # Concurrency
    MAX_OHLC_WORKERS: int = int(os.getenv("MAX_OHLC_WORKERS", "8"))

    # CoinGecko
    COINGECKO_API_KEY: str = os.getenv("COINGECKO_API_KEY", "")
    COINGECKO_BASE_URL: str = "https://api.coingecko.com/api/v3"
    OHLC_DAYS: int = int(os.getenv("OHLC_DAYS", "1"))

    # Persistence
    QUEUE_FILE: str = os.getenv("QUEUE_FILE", "delayed_queue.json")
    SENT_SIGNALS_FILE: str = os.getenv("SENT_SIGNALS_FILE", "sent_signals.json")
    SIGNAL_COOLDOWN_SEC: int = int(os.getenv("SIGNAL_COOLDOWN_SEC", "7200"))

    @classmethod
    def validate(cls) -> bool:
        if not cls.AGENT_PRIVATE_KEY:
            print("ERROR: AGENT_PRIVATE_KEY not set")
            print("Generate one with: python -c \"from eth_account import Account; print(Account.create().key.hex())\"")
            return False
        return True
