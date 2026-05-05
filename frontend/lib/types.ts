// 0G Signal Marketplace API Types

export type Tier = "BRONZE" | "SILVER" | "GOLD" | "DIAMOND"

export type SignalState = 
  | "PENDING" 
  | "ACTIVE" 
  | "CLOSED_WIN" 
  | "CLOSED_LOSS" 
  | "EXPIRED" 
  | "CANCELLED"

export type OutcomeType = "green" | "yellow" | "red" | "gray"

export type Direction = "LONG" | "SHORT"

export interface NodeStats {
  node_id: string
  total_signals: number
  win_count: number
  loss_count: number
  expired_count: number
  win_rate: number
  avg_ev: number
  sharpe_ratio: number
  trust_score: number
  tier: Tier
  updated_at: number
}

export interface TierDistribution {
  BRONZE: number
  SILVER: number
  GOLD: number
  DIAMOND: number
}

export interface DashboardData {
  total_nodes: number
  total_signals: number
  total_wins: number
  total_losses: number
  platform_win_rate: number
  tier_distribution: TierDistribution
  top_nodes: NodeStats[]
  updated_at: number
}

export interface NodesResponse {
  nodes: NodeStats[]
  count: number
}

export interface Signal {
  id: string
  token_pair: string
  direction: Direction
  entry_price: number
  take_profit: number
  stop_loss: number
  closed_price: number
  pnl_percent: number
  outcome: SignalState
  outcome_type: OutcomeType
  confidence: number
  created_at: number
  closed_at: number
  duration_sec: number
}

export interface TokenPerformance {
  token_pair: string
  total_signals: number
  wins: number
  losses: number
  win_rate: number
  avg_pnl: number
  total_pnl: number
}

export interface MonthlyReturn {
  month: string
  pnl_percent: number
  signals: number
  win_rate: number
}

export interface Streaks {
  current_streak: number
  current_streak_type: "winning" | "losing"
  longest_win_streak: number
  longest_loss_streak: number
}

export interface NodeAnalytics {
  node_id: string
  stats: NodeStats
  signal_history: Signal[]
  performance_by_token: Record<string, TokenPerformance>
  monthly_returns: MonthlyReturn[]
  streaks: Streaks
}

export interface EquityPoint {
  timestamp: number
  equity: number
  pnl_percent: number
}

export interface TradeHistoryItem {
  signal_id: string
  token_pair: string
  direction: Direction
  entry_price: number
  exit_price: number
  pnl_percent: number
  pnl_amount: number
  outcome: SignalState
  outcome_type: OutcomeType
  timestamp: number
}

export interface PortfolioSimulation {
  initial_capital: number
  final_value: number
  total_return_percent: number
  max_drawdown_percent: number
  sharpe_ratio: number
  total_trades: number
  timeframe_days: number
  equity_curve: EquityPoint[]
  trade_history: TradeHistoryItem[]
}

// Tier configuration
export const TIER_CONFIG: Record<Tier, { 
  min_score: number
  max_score: number
  monthly_price: number
  creator_split: number
  color: string
}> = {
  BRONZE: { min_score: 0, max_score: 40, monthly_price: 10, creator_split: 0.6, color: "#CD7F32" },
  SILVER: { min_score: 41, max_score: 70, monthly_price: 25, creator_split: 0.7, color: "#C0C0C0" },
  GOLD: { min_score: 71, max_score: 90, monthly_price: 50, creator_split: 0.8, color: "#FFD700" },
  DIAMOND: { min_score: 91, max_score: 100, monthly_price: 100, creator_split: 0.85, color: "#B9F2FF" },
}

export interface RateLimitStats {
  global_requests_in_window: number
  global_max: number
  global_window_sec: number
  node_max: number
  node_window_sec: number
  tracked_nodes: number
  cached_signal_hashes: number
}

export interface SignalSubmission {
  node_id: string
  envelope: {
    node_id: string
    timestamp: number
    payload: SignalPayload
    signature: string
  }
}

export interface SignalPayload {
  token_pair: string
  exchange?: string
  direction: "long" | "short"
  entry_price: number
  take_profit: number
  stop_loss: number
  expiry_time: number
  weight_pct: number
  trade_type?: "spot" | "perpetual" | "futures"
  leverage?: number
  take_profits?: TakeProfitLevel[]
}

export interface TakeProfitLevel {
  price: number
  percent: number
}

export interface SignalResponse {
  status: "accepted" | "rejected"
  message: string
  signal_id?: string
  error?: string
}

export interface WebhookRegistration {
  node_id: string
  target_url: string
  subscriber_address: string
  signature: string
  timestamp: number
}

export interface WebhookResponse {
  status: string
  message: string
}

 export const OUTCOME_COLORS = {
  green: "#16a34a",
  yellow: "#eab308",
  red: "#dc2626",
  gray: "#6b7280",
} as const