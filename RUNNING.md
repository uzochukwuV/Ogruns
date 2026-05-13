# Running the Complete Ogruns System

This guide explains how to run all components of the Ogruns autonomous trading system in production.

## System Architecture

```
┌─────────────────────┐
│  agent-template     │  → Scans markets, generates signals with AI validation
│  (Signal Generator) │     Uses 0G Compute for multi-factor analysis
└──────────┬──────────┘
           │
           │ Posts signals via REST API
           ↓
┌─────────────────────┐
│ verification-layer  │  → Scores agents, verifies signals, streams to clients
│   (Signal Hub)      │     Tracks performance and trust scores
└──────────┬──────────┘
           │
           │ WebSocket stream
           ↓
┌─────────────────────┐
│     Sentinel        │  → Consumes signals, executes trades on GMX/Arbitrum
│  (Trading Agent)    │     Displays positions on web dashboard
└─────────────────────┘
```

## Prerequisites

- Node.js 18+ (for Sentinel and verification-layer)
- Go 1.21+ (for verification-layer scorer)
- Python 3.9+ (for agent-template)
- PostgreSQL (for verification-layer)

## Component 1: Verification Layer (Signal Hub)

### Configuration

Located in `verification-layer/.env`:
```bash
# Database
DATABASE_URL=postgresql://user:pass@localhost:5432/ogruns_signals

# Server
PORT=8080
HOST=0.0.0.0

# AI Scorer (0G Compute)
ZG_AI_API_KEY=sk-bb0bf388-2c61-4cb1-a1ca-100d5e73c7b3
ZG_AI_API_URL=https://router-api-testnet.integratenetwork.work/v1/chat/completions
ZG_AI_MODEL=qwen/qwen-2.5-7b-instruct
```

### Running

```bash
cd verification-layer
go run cmd/server/main.go
```

**Production deployment**: Already deployed at `https://well-noelle-visualisecrypto-fff1956e.koyeb.app`

## Component 2: Agent Template (Signal Generator)

### Configuration

Located in `agent-template/.env`:
```bash
# Agent Identity
AGENT_PRIVATE_KEY=0x3f4055d852285d4e4321973d6ea7783272aa0809566cf82c188a6636092ee0e2

# Verification Layer API
VERIFIER_API_URL=https://well-noelle-visualisecrypto-fff1956e.koyeb.app

# Scan Configuration
MIN_PRICE_CHANGE_PCT=4
MAX_PRICE_CHANGE_PCT=9
SCAN_INTERVAL_SEC=90
MIN_SIGNAL_SCORE=55

# CoinGecko API (optional for free tier)
COINGECKO_API_KEY=CG-R2paEiA7U3zZkjA6zZR7if4W

# 0G Compute AI Analysis
ZG_AI_API_KEY=sk-bb0bf388-2c61-4cb1-a1ca-100d5e73c7b3
ZG_AI_API_URL=https://router-api-testnet.integratenetwork.work/v1/chat/completions
ZG_AI_MODEL=qwen/qwen-2.5-7b-instruct
AI_MIN_CONFIDENCE=60
AI_ENABLED=true
```

### AI Analysis Features

The agent uses 0G Compute to analyze signals with these tools:
- **Hourly Patterns**: 7-day historical analysis to identify best/worst trading hours
- **Volume Analysis**: Detect volume spikes or low conviction moves
- **Volatility Analysis**: Check if TP/SL targets are realistic given recent volatility
- **BTC Correlation**: For altcoins, compare signal direction with BTC trend
- **Market Sentiment**: Fear & Greed Index to gauge overall market conditions
- **Recent Price Action**: Current price trends (1h, 24h, 7d)

Signals must pass AI validation with ≥60% confidence to be submitted.

### Running

```bash
cd agent-template
pip install -r requirements.txt
python broadcaster.py
```

**Options**:
```bash
# One-shot scan (no loop)
python broadcaster.py --once

# Custom thresholds
python broadcaster.py --min 5 --max 8 --interval 120

# Disable AI validation
python broadcaster.py --no-ai
```

### Testing AI Tools

```bash
# Test data fetching (hourly patterns, volume, sentiment, etc.)
python test_fetching.py

# Test AI analyzer directly
python ai_analyzer.py
```

## Component 3: Sentinel (Trading Agent)

### Configuration

Located in `sentinel/.env`:
```bash
# Wallet
PRIVATE_KEY=0x3f4055d852285d4e4321973d6ea7783272aa0809566cf82c188a6636092ee0e2

# Signal Platform
SIGNAL_API_URL=https://well-noelle-visualisecrypto-fff1956e.koyeb.app
SIGNAL_WS_URL=ws://well-noelle-visualisecrypto-fff1956e.koyeb.app/api/v1/stream/public

# GMX Configuration (Arbitrum Sepolia testnet)
ARBITRUM_RPC_URL=https://sepolia-rollup.arbitrum.io/rpc
CHAIN_ID=421614

# Trading Parameters
MIN_TRUST_SCORE=10
MIN_TIER=BRONZE
MAX_POSITION_USD=10
COLLATERAL_TOKEN=USDC

# Risk Management
MAX_OPEN_POSITIONS=3
AUTO_STOP_LOSS_PCT=5
AUTO_TAKE_PROFIT_PCT=10
```

### Running

```bash
cd sentinel
npm install
npm run dev
```

**Dashboard**: Opens at `http://localhost:3000` showing:
- Active positions with live PnL
- Recent trades
- Signal history
- Agent statistics

### Production Build

```bash
npm run build
npm start
```

## Complete System Startup

### Option 1: Development (All Local)

Terminal 1 - Verification Layer:
```bash
cd verification-layer
go run cmd/server/main.go
```

Terminal 2 - Agent Template:
```bash
cd agent-template
python broadcaster.py
```

Terminal 3 - Sentinel:
```bash
cd sentinel
npm run dev
```

### Option 2: Production (Using Deployed Verifier)

Terminal 1 - Agent Template:
```bash
cd agent-template
python broadcaster.py
```

Terminal 2 - Sentinel:
```bash
cd sentinel
npm start
```

**Note**: Verification layer is already deployed at the Koyeb URL.

## Monitoring

### Agent Template Logs

```
[13:19:16] Scanning for coins with 4.0-9.0% 24h change...
[13:19:17] Found 6 coins. Fetching OHLC data...
[13:19:18] Fetched OHLC candles for 6/6 qualified coin(s)

SYMBOL            PRICE     24H%        VOL  SCORE      PAT  STATUS
---------------------------------------------------------------------------
JTO       $       0.53   +6.89%     39.9M   100 INV,SHO,HAM  VERY STRONG
TIA       $       0.49   +8.62%    143.6M   100 BEA,DAR,THR  VERY STRONG
DOT       $       1.39   +4.07%    178.8M    85 DOJ,DOJ,DOJ  STRONG

[13:19:25]   -> AI analyzing DOT/USDT SHORT...
[AI] Calling tool: get_hourly_patterns(DOT/USDT)
[AI] Calling tool: get_volume_analysis(DOT/USDT)
[AI] Calling tool: get_btc_correlation(DOT/USDT)
[13:19:47]   -> AI APPROVED: confidence 75%
[13:19:47] Submitting SHORT DOT/USDT (score: 85%, expiry: 100m)
[13:19:48]   -> Queued: 0x2aBA...960E4_1778674787
```

### Sentinel Logs

```
💰 Starting in LIVE TRADING mode...

[13:20:15] 🔗 Connected to signal stream
[13:20:15] 📊 Wallet: 0x2aBA...960E4
[13:20:15] 💵 USDC Balance: 50.00
[13:20:16] 📡 New signal: DOT/USDT SHORT (score: 85)
[13:20:16]   ✓ Passed filters (trust: 95, tier: GOLD)
[13:20:17]   📈 Opening position: $10.00 @ $1.39
[13:20:18]   ✅ Position opened: txHash 0xabc...123
[13:20:18] 💼 Active positions: 1 | Total PnL: +$0.00
```

### Dashboard View

Navigate to `http://localhost:3000`:
- **Positions Panel**: Shows all active positions with:
  - Token pair, direction (LONG/SHORT)
  - Entry price, current price
  - PnL (USD and %)
  - Position size
- **Trades Panel**: Recent completed trades
- **Stats**: Total PnL, win rate, active positions

## Key Features

### Agent Template (Signal Generator)
✅ Multi-factor AI analysis using 0G Compute
✅ Hourly pattern analysis (7-day historical data)
✅ Volume spike detection
✅ BTC correlation checks for altcoins
✅ Market sentiment integration (Fear & Greed)
✅ Volatility-based TP/SL validation
✅ Automatic signal cooldown (2 hours per token)
✅ Duplicate signal prevention

### Sentinel (Trading Agent)
✅ Real-time signal streaming via WebSocket
✅ Trust score filtering (only follow high-quality agents)
✅ Automatic position management
✅ Live PnL tracking and dashboard
✅ GMX SDK integration for perpetual futures
✅ Risk management (max positions, auto SL/TP)

### Verification Layer (Signal Hub)
✅ EIP-191 signature verification
✅ Performance tracking (WIN/LOSS state)
✅ Dynamic trust scoring
✅ Public signal stream (WebSocket)
✅ Historical signal API

## Troubleshooting

### Agent Template Not Finding Signals
- Check `MIN_PRICE_CHANGE_PCT` and `MAX_PRICE_CHANGE_PCT` range
- Lower `MIN_SIGNAL_SCORE` if being too strict
- Check CoinGecko API rate limits

### AI Analyzer Falling Back to Heuristics
- Verify `ZG_AI_API_KEY` is set correctly
- Check 0G Compute API is responding: `curl -H "Authorization: Bearer $ZG_AI_API_KEY" https://router-api-testnet.integratenetwork.work/v1/models`
- Run `python test_fetching.py` to verify data tools work

### Sentinel Not Opening Positions
- Check wallet has USDC on Arbitrum Sepolia
- Verify `MIN_TRUST_SCORE` and `MIN_TIER` settings
- Check GMX market conditions (liquidity, open interest)
- Review logs for position size errors

### Dashboard Not Showing Positions
- Wait 30s for sync interval
- Check browser console for WebSocket errors
- Verify Sentinel is connected to signal stream
- Restart Sentinel if needed

## Next Steps

1. **Improve AI Decisioning**: Increase `max_iterations` in [ai_analyzer.py:45](agent-template/ai_analyzer.py#L45) if needed
2. **Add More Tokens**: Extend `_get_coingecko_id()` mapping in [ai_analyzer.py:556](agent-template/ai_analyzer.py#L556)
3. **Backtest**: Use signal history API to analyze agent performance
4. **Scale**: Deploy agent-template to run 24/7 on server
5. **Monitor**: Set up alerts for large PnL swings or position failures

## Support

For issues:
- Agent Template: Check `agent-template/sent_signals.json` for recent signals
- Sentinel: Check browser console and terminal logs
- Verification Layer: Check `/api/v1/health` endpoint

---

**System Status**: ✅ All components tested and functional
**Last Updated**: 2026-05-13
