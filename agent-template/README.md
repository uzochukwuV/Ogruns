# AI Trading Signal Agent Template

A Python template for building AI trading signal agents that publish to the 0G Signal Intelligence Network.

## Quick Start

```bash
# 1. Install dependencies
pip install -r requirements.txt

# 2. Generate a new agent key
python -c "from eth_account import Account; a=Account.create(); print(f'Address: {a.address}\nPrivate Key: {a.key.hex()}')"

# 3. Create .env file
cp .env.example .env
# Edit .env and add your AGENT_PRIVATE_KEY

# 4. Register your agent on 0G Network
python scripts/register_agent.py --network mainnet --name "My Agent" --description "BTC/ETH signal specialist"

# 5. Run the agent
python broadcaster.py
```

## Agent Registration

Before submitting signals, you must register your agent on the AgentRegistry smart contract.

### Register on Mainnet

```bash
python scripts/register_agent.py --network mainnet \
  --name "My Trading Agent" \
  --description "AI-powered crypto signal provider specializing in BTC/ETH"
```

### Register on Testnet

```bash
python scripts/register_agent.py --network testnet \
  --name "Test Agent" \
  --description "Testing agent"
```

### Check Registration Status

```bash
python scripts/register_agent.py --check
```

### Network Details

| Network | Chain ID | RPC URL | Explorer |
|---------|----------|---------|----------|
| Mainnet | 16600 | `https://rpc-mainnet.0g.ai` | [chainscan.0g.ai](https://chainscan.0g.ai) |
| Testnet | 16600 | `https://rpc-testnet.0g.ai` | [explorer-testnet.0g.ai](https://explorer-testnet.0g.ai) |

**Note**: You need 0G tokens for gas fees. Get them from:
- **Mainnet**: Exchange or bridge
- **Testnet**: [Faucet](https://faucet.0g.ai)

## Architecture

```
agent-template/
├── broadcaster.py   # Main daemon - parallel OHLC fetch, delayed analysis, queue persistence
├── scanner.py       # CoinGecko market data fetcher with candlestick pattern detection
├── generator.py     # Weight-based signal generator with reversal scoring
├── signer.py        # EIP-191 signature creation
├── config.py        # Configuration loader
└── requirements.txt
```

## Three-Tier Confidence System

| Tier | Score Range | Action | Delay |
|------|-------------|--------|-------|
| WATCH | 55-59 | Monitor only | - |
| MILD | 60-69 | Scheduled for delayed analysis | 15 min |
| MODERATE | 70-79 | Scheduled for delayed analysis | 30 min |
| STRONG | 80-89 | Immediate signal | - |
| VERY STRONG | 90-100 | High-priority signal | - |

## Candlestick Pattern Detection

| Pattern | Type | Score Impact | Description |
|---------|------|--------------|-------------|
| Hammer | Bullish | +15 | Small body, long lower wick at downtrend bottom |
| Shooting Star | Bearish | +12 | Small body, long upper wick at uptrend top |
| Bullish Engulfing | Bullish | +20 | Larger bullish candle engulfs previous bearish |
| Bearish Engulfing | Bearish | +20 | Larger bearish candle engulfs previous bullish |
| Piercing Line | Bullish | +15 | 2-candle reversal after downtrend |
| Dark Cloud Cover | Bearish | +15 | 2-candle reversal after uptrend |
| Morning Star | Bullish | +25 | 3-candle bullish reversal pattern |
| Three Black Crows | Bearish | +20 | 3 consecutive bearish candles |
| Doji | Indecision | ±5 | Very small body, market uncertainty |

## Signal Generation Logic

The agent uses a **reversal-based strategy**:

| Factor | Score Impact | Description |
|--------|--------------|-------------|
| Momentum Decay | +20 | 1h pace slowing vs 24h average |
| Volume Spike | +15 | Daily vol > 15% of market cap |
| Timeframe Divergence | +15 | 1h/24h/7d not pointing same direction |
| Overextension | +10 | >7% move in 24h |
| Near ATH (bull) | -10 | Less reversal room |
| Bearish Patterns (bull trend) | +varies | Confirms reversal likelihood |
| Bullish Patterns (bear trend) | +varies | Confirms reversal likelihood |

**Signal Direction:**
- BULL trend + high reversal score → **SHORT** (expect reversal down)
- BEAR trend + high reversal score → **LONG** (expect reversal up)

## Configuration

| Env Variable | Default | Description |
|--------------|---------|-------------|
| `AGENT_PRIVATE_KEY` | (required) | Agent's signing key |
| `VERIFIER_API_URL` | `http://localhost:8080` | Verifier API endpoint |
| `MIN_PRICE_CHANGE_PCT` | `2` | Minimum 24h price change |
| `MAX_PRICE_CHANGE_PCT` | `8` | Maximum 24h price change |
| `MIN_VOLUME_USD` | `5000000` | Minimum 24h volume (USD) |
| `SCAN_INTERVAL_SEC` | `90` | Seconds between scans |
| `DELAYED_ANALYSIS_MILD_DELAY` | `900` | MILD tier delay (15 min) |
| `DELAYED_ANALYSIS_MODERATE_DELAY` | `1800` | MODERATE tier delay (30 min) |
| `MAX_OHLC_WORKERS` | `8` | Parallel OHLC fetch threads |
| `QUEUE_FILE` | `delayed_queue.json` | Queue persistence file |
| `COINGECKO_API_KEY` | (optional) | CoinGecko API key |

## CLI Options

```bash
# One-shot scan
python broadcaster.py --once

# Custom thresholds
python broadcaster.py --min 3 --max 10 --interval 300

# Higher signal threshold
python broadcaster.py --min-score 65
```

## Display Output

```
SYMBOL        PRICE     24H%       VOL    SCORE  PAT       STATUS
-------------------------------------------------------------------------------
BTC      $ 65000.00  +5.20%      5.2M    92 HAM,ENG   VERY STRONG
ETH      $  3200.00  -4.80%      2.1M    75        -      MODERATE
SOL      $  140.00   +8.10%      1.8M    62 DOJI      MILD
```

## Key Features

- **Parallel OHLC Fetching**: Uses ThreadPoolExecutor for concurrent API calls
- **Volume Filtering**: Skips low-liquidity tokens (configurable threshold)
- **Persistent Queue**: Delayed analysis items survive restarts
- **Enhanced Patterns**: Doji, Shooting Star, Piercing Line, Dark Cloud Cover
- **Three-Tier Confidence**: Better signal quality control

## Extending the Agent

### Custom Signal Strategy

Edit `generator.py` to implement your own scoring logic:

```python
def compute_reversal_score(coin_data: Dict) -> Dict:
    score = 50
    # Add RSI, MACD, or other indicators
    return {"score": score, ...}
```

## Signal Payload Format

```json
{
  "token_pair": "BTC/USDT",
  "exchange": "coingecko",
  "direction": "long",
  "entry_price": 65000.0,
  "take_profit": 68000.0,
  "stop_loss": 63000.0,
  "expiry_time": 1699999999,
  "weight_pct": 75,
  "trade_type": "spot",
  "leverage": 1
}
```

## License

MIT - Part of the 0G Signal Intelligence Network