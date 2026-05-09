import WebSocket from "ws";
import type { ActiveSignal, NodeStats, AgentConfig, TIER_PRIORITY } from "../core/types.js";

export type SignalHandler = (signal: ActiveSignal, nodeStats: NodeStats | null) => Promise<void>;

interface DashboardResponse {
  total_nodes: number;
  total_signals: number;
  total_wins: number;
  total_losses: number;
  platform_win_rate: number;
  tier_distribution: Record<string, number>;
  top_nodes: NodeStats[];
  updated_at: number;
}

export class SignalConsumer {
  private ws: WebSocket | null = null;
  private config: AgentConfig;
  private nodeStatsCache: Map<string, NodeStats> = new Map();
  private onSignal: SignalHandler | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private reconnectDelay = 5000;

  constructor(config: AgentConfig) {
    this.config = config;
  }

  // Register signal handler
  onNewSignal(handler: SignalHandler): void {
    this.onSignal = handler;
  }

  // Fetch initial node stats from API
  async fetchNodeStats(): Promise<void> {
    try {
      const response = await fetch(`${this.config.signalApiUrl}/api/v1/analytics/dashboard`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const data = await response.json() as DashboardResponse;

      // Cache node stats
      this.nodeStatsCache.clear();
      for (const node of data.top_nodes) {
        this.nodeStatsCache.set(node.node_id, node);
      }

      console.log(`📊 Loaded ${data.top_nodes.length} nodes from platform`);
      console.log(`   Platform Win Rate: ${(data.platform_win_rate * 100).toFixed(1)}%`);
      console.log(`   Total Signals: ${data.total_signals}`);
    } catch (error) {
      console.error("Failed to fetch node stats:", error);
    }
  }

  // Get cached node stats
  getNodeStats(nodeId: string): NodeStats | null {
    return this.nodeStatsCache.get(nodeId) || null;
  }

  // Fetch active signals from API (for catching up on missed signals)
  async fetchActiveSignals(): Promise<ActiveSignal[]> {
    try {
      const response = await fetch(`${this.config.signalApiUrl}/api/v1/signals/active`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const data = await response.json() as { signals: ActiveSignal[] };
      const signals = data.signals;

      if (signals.length > 0) {
        console.log(`📥 Fetched ${signals.length} active signal(s) from platform`);
      }

      return signals;
    } catch (error) {
      console.error("Failed to fetch active signals:", error);
      return [];
    }
  }

  // Process existing active signals (for newly connected agents)
  async processExistingSignals(): Promise<void> {
    const signals = await this.fetchActiveSignals();

    for (const signal of signals) {
      if (signal.state === "ACTIVE") {
        console.log(`\n📡 Existing Signal: ${signal.envelope.payload.token_pair}`);
        console.log(`   Direction: ${signal.envelope.payload.direction.toUpperCase()}`);
        console.log(`   Entry: $${signal.envelope.payload.entry_price}`);
        console.log(`   Node: ${signal.envelope.node_id.slice(0, 10)}...`);

        const { follow, reason } = this.shouldFollowSignal(signal);

        if (follow) {
          console.log(`   ✅ FOLLOWING: ${reason}`);
          if (this.onSignal) {
            const nodeStats = this.getNodeStats(signal.envelope.node_id);
            await this.onSignal(signal, nodeStats);
          }
        } else {
          console.log(`   ⏭️  SKIPPING: ${reason}`);
        }
      }
    }
  }

  // Check if signal passes our filters
  shouldFollowSignal(signal: ActiveSignal): { follow: boolean; reason: string } {
    const nodeStats = this.getNodeStats(signal.envelope.node_id);

    if (!nodeStats) {
      return { follow: false, reason: "Node not in cache (unknown reputation)" };
    }

    // Check trust score
    if (nodeStats.trust_score < this.config.minTrustScore) {
      return {
        follow: false,
        reason: `Trust score ${nodeStats.trust_score.toFixed(1)} < ${this.config.minTrustScore}`
      };
    }

    // Check tier
    const tierPriority: Record<string, number> = { BRONZE: 1, SILVER: 2, GOLD: 3, DIAMOND: 4 };
    const nodeTierPriority = tierPriority[nodeStats.tier] || 0;
    const minTierPriority = tierPriority[this.config.minTier] || 0;

    if (nodeTierPriority < minTierPriority) {
      return {
        follow: false,
        reason: `Tier ${nodeStats.tier} < ${this.config.minTier}`
      };
    }

    // Check minimum signals for reliability
    if (nodeStats.total_signals < 5) {
      return {
        follow: false,
        reason: `Only ${nodeStats.total_signals} signals (need 5+ for reliability)`
      };
    }

    return { follow: true, reason: "Passed all filters" };
  }

  // Connect to WebSocket stream
  connect(): void {
    console.log(`🔌 Connecting to ${this.config.signalWsUrl}...`);

    this.ws = new WebSocket(this.config.signalWsUrl);

    this.ws.on("open", () => {
      console.log("✅ WebSocket connected to signal stream");
      this.reconnectAttempts = 0;
    });

    this.ws.on("message", async (data: WebSocket.Data) => {
      try {
        const message = JSON.parse(data.toString());
        console.log("📨 WS Message received:", JSON.stringify(message, null, 2));

        if (message.type === "signal_update" && message.data) {
          const signal = message.data as ActiveSignal;

          // Only process new active signals
          if (signal.state === "ACTIVE") {
            console.log(`\n📡 New Signal: ${signal.envelope.payload.token_pair}`);
            console.log(`   Direction: ${signal.envelope.payload.direction.toUpperCase()}`);
            console.log(`   Entry: $${signal.envelope.payload.entry_price}`);
            console.log(`   Node: ${signal.envelope.node_id.slice(0, 10)}...`);

            // Check filters
            const { follow, reason } = this.shouldFollowSignal(signal);

            if (follow) {
              console.log(`   ✅ FOLLOWING: ${reason}`);

              if (this.onSignal) {
                const nodeStats = this.getNodeStats(signal.envelope.node_id);
                await this.onSignal(signal, nodeStats);
              }
            } else {
              console.log(`   ⏭️  SKIPPING: ${reason}`);
            }
          }
        }
      } catch (error) {
        console.error("Failed to parse WebSocket message:", error);
      }
    });

    this.ws.on("close", () => {
      console.log("🔌 WebSocket disconnected");
      this.scheduleReconnect();
    });

    this.ws.on("error", (error) => {
      console.error("WebSocket error:", error);
    });
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error("Max reconnection attempts reached. Giving up.");
      return;
    }

    this.reconnectAttempts++;
    const delay = this.reconnectDelay * this.reconnectAttempts;

    console.log(`Reconnecting in ${delay / 1000}s... (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);

    setTimeout(() => {
      this.connect();
    }, delay);
  }

  // Disconnect
  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}
