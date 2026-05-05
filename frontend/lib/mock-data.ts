import type {
  DashboardData,
  NodeStats,
  NodeAnalytics,
  Signal,
  PortfolioSimulation,
  Tier,
  OutcomeType,
} from "./types"

// Generate deterministic mock node addresses
function generateNodeId(index: number): string {
  const chars = "0123456789abcdef"
  let hash = ""
  for (let i = 0; i < 40; i++) {
    hash += chars[(index * (i + 1) * 7) % 16]
  }
  return `0x${hash.slice(0, 40)}`
}

// Generate mock node stats
function generateNodeStats(index: number): NodeStats {
  const tiers: Tier[] = ["BRONZE", "SILVER", "GOLD", "DIAMOND"]
  const tierWeights = [0.6, 0.24, 0.12, 0.04] // Distribution
  
  let tierIndex = 0
  const rand = ((index * 13) % 100) / 100
  let cumulative = 0
  for (let i = 0; i < tierWeights.length; i++) {
    cumulative += tierWeights[i]
    if (rand < cumulative) {
      tierIndex = i
      break
    }
  }
  
  const tier = tiers[tierIndex]
  const baseScore = tierIndex === 0 ? 20 : tierIndex === 1 ? 55 : tierIndex === 2 ? 80 : 95
  const trustScore = baseScore + ((index * 7) % 15) - 5
  
  const totalSignals = 50 + ((index * 23) % 200)
  const winRate = 0.45 + ((index * 11) % 40) / 100
  const winCount = Math.floor(totalSignals * winRate)
  const lossCount = Math.floor(totalSignals * (1 - winRate) * 0.8)
  const expiredCount = totalSignals - winCount - lossCount
  
  return {
    node_id: generateNodeId(index),
    total_signals: totalSignals,
    win_count: winCount,
    loss_count: lossCount,
    expired_count: expiredCount,
    win_rate: winRate,
    avg_ev: 0.8 + ((index * 17) % 100) / 100,
    sharpe_ratio: 1.2 + ((index * 19) % 150) / 100,
    trust_score: Math.min(100, Math.max(0, trustScore)),
    tier,
    updated_at: Date.now() / 1000 - ((index * 3600) % 86400),
  }
}

// Generate all nodes
const ALL_NODES: NodeStats[] = Array.from({ length: 25 }, (_, i) => generateNodeStats(i))
  .sort((a, b) => b.trust_score - a.trust_score)

// Dashboard data
export const MOCK_DASHBOARD: DashboardData = {
  total_nodes: ALL_NODES.length,
  total_signals: ALL_NODES.reduce((sum, n) => sum + n.total_signals, 0),
  total_wins: ALL_NODES.reduce((sum, n) => sum + n.win_count, 0),
  total_losses: ALL_NODES.reduce((sum, n) => sum + n.loss_count, 0),
  platform_win_rate: 0.627,
  tier_distribution: {
    BRONZE: ALL_NODES.filter(n => n.tier === "BRONZE").length,
    SILVER: ALL_NODES.filter(n => n.tier === "SILVER").length,
    GOLD: ALL_NODES.filter(n => n.tier === "GOLD").length,
    DIAMOND: ALL_NODES.filter(n => n.tier === "DIAMOND").length,
  },
  top_nodes: ALL_NODES.slice(0, 10),
  updated_at: Date.now() / 1000,
}

export const MOCK_NODES = ALL_NODES

// Generate signal history for a node
function generateSignalHistory(nodeId: string, count: number): Signal[] {
  const pairs = ["BTC/USDT", "ETH/USDT", "SOL/USDT", "AVAX/USDT", "ARB/USDT"]
  const signals: Signal[] = []
  
  const now = Date.now() / 1000
  for (let i = 0; i < count; i++) {
    const direction = i % 3 === 0 ? "SHORT" : "LONG"
    const pairIndex = (i * 7) % pairs.length
    const pair = pairs[pairIndex]
    
    // Base prices per pair
    const basePrices: Record<string, number> = {
      "BTC/USDT": 64500,
      "ETH/USDT": 3200,
      "SOL/USDT": 145,
      "AVAX/USDT": 35,
      "ARB/USDT": 1.2,
    }
    const basePrice = basePrices[pair]
    const entryPrice = basePrice * (0.98 + ((i * 13) % 5) / 100)
    
    const outcomeRand = (i * 11) % 100
    let outcome: Signal["outcome"]
    let outcomeType: OutcomeType
    let pnlPercent: number
    let closedPrice: number
    
    if (outcomeRand < 55) {
      outcome = "CLOSED_WIN"
      outcomeType = "green"
      pnlPercent = 1.5 + ((i * 7) % 50) / 10
      closedPrice = direction === "LONG" 
        ? entryPrice * (1 + pnlPercent / 100)
        : entryPrice * (1 - pnlPercent / 100)
    } else if (outcomeRand < 75) {
      outcome = "EXPIRED"
      outcomeType = outcomeRand < 65 ? "yellow" : "red"
      pnlPercent = outcomeRand < 65 
        ? 0.5 + ((i * 3) % 20) / 10 
        : -(0.5 + ((i * 3) % 20) / 10)
      closedPrice = direction === "LONG"
        ? entryPrice * (1 + pnlPercent / 100)
        : entryPrice * (1 - pnlPercent / 100)
    } else {
      outcome = "CLOSED_LOSS"
      outcomeType = "red"
      pnlPercent = -(1 + ((i * 5) % 40) / 10)
      closedPrice = direction === "LONG"
        ? entryPrice * (1 + pnlPercent / 100)
        : entryPrice * (1 - pnlPercent / 100)
    }
    
    const createdAt = now - (i * 3600) - ((i * 1234) % 7200)
    const durationSec = 1800 + ((i * 567) % 7200)
    
    signals.push({
      id: `${nodeId}_${Math.floor(createdAt)}`,
      token_pair: pair,
      direction,
      entry_price: entryPrice,
      take_profit: direction === "LONG" ? entryPrice * 1.04 : entryPrice * 0.96,
      stop_loss: direction === "LONG" ? entryPrice * 0.97 : entryPrice * 1.03,
      closed_price: closedPrice,
      pnl_percent: pnlPercent,
      outcome,
      outcome_type: outcomeType,
      confidence: 65 + ((i * 11) % 30),
      created_at: createdAt,
      closed_at: createdAt + durationSec,
      duration_sec: durationSec,
    })
  }
  
  return signals.sort((a, b) => b.created_at - a.created_at)
}

// Get node analytics
export function getMockNodeAnalytics(nodeId: string): NodeAnalytics | null {
  const node = ALL_NODES.find(n => n.node_id === nodeId)
  if (!node) return null
  
  const signals = generateSignalHistory(nodeId, node.total_signals)
  
  // Calculate performance by token
  const performanceByToken: NodeAnalytics["performance_by_token"] = {}
  const pairs = ["BTC/USDT", "ETH/USDT", "SOL/USDT", "AVAX/USDT", "ARB/USDT"]
  
  for (const pair of pairs) {
    const pairSignals = signals.filter(s => s.token_pair === pair)
    if (pairSignals.length === 0) continue
    
    const wins = pairSignals.filter(s => s.outcome_type === "green" || (s.outcome_type === "yellow" && s.pnl_percent > 0))
    const losses = pairSignals.filter(s => s.outcome_type === "red")
    const totalPnl = pairSignals.reduce((sum, s) => sum + s.pnl_percent, 0)
    
    performanceByToken[pair] = {
      token_pair: pair,
      total_signals: pairSignals.length,
      wins: wins.length,
      losses: losses.length,
      win_rate: wins.length / pairSignals.length,
      avg_pnl: totalPnl / pairSignals.length,
      total_pnl: totalPnl,
    }
  }
  
  // Monthly returns
  const monthlyReturns = [
    { month: "2024-01", pnl_percent: 8.5 + (Math.random() * 10 - 5), signals: 35 + Math.floor(Math.random() * 20), win_rate: 0.6 + Math.random() * 0.15 },
    { month: "2024-02", pnl_percent: 12.3 + (Math.random() * 10 - 5), signals: 42 + Math.floor(Math.random() * 20), win_rate: 0.65 + Math.random() * 0.1 },
    { month: "2024-03", pnl_percent: 5.8 + (Math.random() * 10 - 5), signals: 38 + Math.floor(Math.random() * 20), win_rate: 0.55 + Math.random() * 0.15 },
    { month: "2024-04", pnl_percent: 15.2 + (Math.random() * 10 - 5), signals: 48 + Math.floor(Math.random() * 20), win_rate: 0.7 + Math.random() * 0.1 },
  ]
  
  // Streaks
  let currentStreak = 0
  let currentStreakType: "winning" | "losing" = "winning"
  let longestWin = 0
  let longestLoss = 0
  let tempWin = 0
  let tempLoss = 0
  
  for (const signal of signals.slice().reverse()) {
    if (signal.pnl_percent > 0) {
      tempWin++
      if (tempLoss > longestLoss) longestLoss = tempLoss
      tempLoss = 0
    } else {
      tempLoss++
      if (tempWin > longestWin) longestWin = tempWin
      tempWin = 0
    }
  }
  if (tempWin > longestWin) longestWin = tempWin
  if (tempLoss > longestLoss) longestLoss = tempLoss
  
  // Current streak
  for (const signal of signals) {
    if (currentStreak === 0) {
      currentStreak = 1
      currentStreakType = signal.pnl_percent > 0 ? "winning" : "losing"
    } else if ((signal.pnl_percent > 0 && currentStreakType === "winning") ||
               (signal.pnl_percent <= 0 && currentStreakType === "losing")) {
      currentStreak++
    } else {
      break
    }
  }
  
  return {
    node_id: nodeId,
    stats: node,
    signal_history: signals,
    performance_by_token: performanceByToken,
    monthly_returns: monthlyReturns,
    streaks: {
      current_streak: currentStreak,
      current_streak_type: currentStreakType,
      longest_win_streak: longestWin,
      longest_loss_streak: longestLoss,
    },
  }
}

// Portfolio simulation
export function getMockPortfolioSimulation(
  nodeId: string,
  initialCapital: number = 1000,
  fromTimestamp?: number,
  toTimestamp?: number
): PortfolioSimulation | null {
  const analytics = getMockNodeAnalytics(nodeId)
  if (!analytics) return null
  
  let signals = analytics.signal_history
  
  // Filter by time range
  if (fromTimestamp) {
    signals = signals.filter(s => s.created_at >= fromTimestamp)
  }
  if (toTimestamp) {
    signals = signals.filter(s => s.created_at <= toTimestamp)
  }
  
  // Sort chronologically
  signals = signals.sort((a, b) => a.created_at - b.created_at)
  
  let equity = initialCapital
  let maxEquity = equity
  let maxDrawdown = 0
  
  const equityCurve: PortfolioSimulation["equity_curve"] = [
    { timestamp: signals[0]?.created_at || Date.now() / 1000 - 86400 * 90, equity: initialCapital, pnl_percent: 0 }
  ]
  
  const tradeHistory: PortfolioSimulation["trade_history"] = []
  
  for (const signal of signals) {
    const pnlAmount = equity * (signal.pnl_percent / 100)
    equity += pnlAmount
    
    if (equity > maxEquity) maxEquity = equity
    const drawdown = ((maxEquity - equity) / maxEquity) * 100
    if (drawdown > maxDrawdown) maxDrawdown = drawdown
    
    equityCurve.push({
      timestamp: signal.closed_at,
      equity,
      pnl_percent: ((equity - initialCapital) / initialCapital) * 100,
    })
    
    tradeHistory.push({
      signal_id: signal.id,
      token_pair: signal.token_pair,
      direction: signal.direction,
      entry_price: signal.entry_price,
      exit_price: signal.closed_price,
      pnl_percent: signal.pnl_percent,
      pnl_amount: pnlAmount,
      outcome: signal.outcome,
      outcome_type: signal.outcome_type,
      timestamp: signal.created_at,
    })
  }
  
  const firstTimestamp = signals[0]?.created_at || Date.now() / 1000 - 86400 * 90
  const lastTimestamp = signals[signals.length - 1]?.closed_at || Date.now() / 1000
  const timeframeDays = Math.ceil((lastTimestamp - firstTimestamp) / 86400)
  
  return {
    initial_capital: initialCapital,
    final_value: equity,
    total_return_percent: ((equity - initialCapital) / initialCapital) * 100,
    max_drawdown_percent: maxDrawdown,
    sharpe_ratio: analytics.stats.sharpe_ratio,
    total_trades: signals.length,
    timeframe_days: timeframeDays,
    equity_curve: equityCurve,
    trade_history: tradeHistory,
  }
}
