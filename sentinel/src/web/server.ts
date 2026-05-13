import express, { Request, Response } from "express";
import { createServer } from "http";
import { WebSocketServer, WebSocket } from "ws";
import path from "path";
import { fileURLToPath } from "url";
import type { ActiveSignal, TradeExecution, NodeStats } from "../core/types.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Dashboard state shared with agent
export interface DashboardState {
  // Wallet
  ethBalance: string;
  collateralBalance: string;
  collateralSymbol: string;
  walletAddress: string;

  // Trading stats
  signalsReceived: number;
  signalsFollowed: number;
  tradesExecuted: number;
  tradesFailed: number;

  // Positions & PnL
  activePositions: number;
  maxPositions: number;
  totalPnL: number;

  // Mode & config
  paperMode: boolean;
  chainId: number;
  minTrustScore: number;
  minTier: string;
  maxPositionUsd: number;

  // Recent activity
  recentSignals: SignalEvent[];
  recentTrades: TradeEvent[];
  positions: PositionInfo[];

  // Platform stats
  platformWinRate: number;
  totalNodes: number;

  // Connection status
  wsConnected: boolean;
  lastUpdate: number;
}

export interface SignalEvent {
  id: string;
  tokenPair: string;
  direction: string;
  entryPrice: number;
  nodeId: string;
  nodeTier: string;
  trustScore: number;
  followed: boolean;
  reason: string;
  timestamp: number;
}

export interface TradeEvent {
  signalId: string;
  symbol: string;
  direction: string;
  sizeUsd: number;
  status: string;
  txHash?: string;
  error?: string;
  timestamp: number;
}

export interface PositionInfo {
  symbol: string;
  direction: string;
  sizeUsd: number;
  pnl: number;
  entryPrice: number;
}

export class DashboardServer {
  private app: express.Application;
  private server: ReturnType<typeof createServer>;
  private wss: WebSocketServer;
  private state: DashboardState;
  private clients: Set<WebSocket> = new Set();
  private port: number;

  constructor(port: number = 3000) {
    this.port = port;
    this.app = express();
    this.server = createServer(this.app);
    this.wss = new WebSocketServer({ server: this.server });

    this.state = this.getInitialState();
    this.setupRoutes();
    this.setupWebSocket();
  }

  private getInitialState(): DashboardState {
    return {
      ethBalance: "0",
      collateralBalance: "0",
      collateralSymbol: "USDC",
      walletAddress: "",
      signalsReceived: 0,
      signalsFollowed: 0,
      tradesExecuted: 0,
      tradesFailed: 0,
      activePositions: 0,
      maxPositions: 3,
      totalPnL: 0,
      paperMode: false,
      chainId: 421614,
      minTrustScore: 50,
      minTier: "SILVER",
      maxPositionUsd: 10,
      recentSignals: [],
      recentTrades: [],
      positions: [],
      platformWinRate: 0,
      totalNodes: 0,
      wsConnected: false,
      lastUpdate: Date.now()
    };
  }

  private setupRoutes(): void {
    // Serve static files from public directory
    this.app.use(express.static(path.join(__dirname, "../../public")));

    // API endpoints
    this.app.get("/api/state", (_req: Request, res: Response) => {
      res.json(this.state);
    });

    this.app.get("/api/signals", (_req: Request, res: Response) => {
      res.json(this.state.recentSignals);
    });

    this.app.get("/api/trades", (_req: Request, res: Response) => {
      res.json(this.state.recentTrades);
    });

    this.app.get("/api/positions", (_req: Request, res: Response) => {
      res.json(this.state.positions);
    });

    // Health check
    this.app.get("/api/health", (_req: Request, res: Response) => {
      res.json({ status: "ok", uptime: process.uptime() });
    });

    // Serve dashboard HTML
    this.app.get("/", (_req: Request, res: Response) => {
      res.sendFile(path.join(__dirname, "../../public/index.html"));
    });
  }

  private setupWebSocket(): void {
    this.wss.on("connection", (ws: WebSocket) => {
      console.log("📊 Dashboard client connected");
      this.clients.add(ws);

      // Send initial state
      ws.send(JSON.stringify({ type: "state", data: this.state }));

      ws.on("close", () => {
        console.log("📊 Dashboard client disconnected");
        this.clients.delete(ws);
      });

      ws.on("error", (error) => {
        console.error("Dashboard WS error:", error);
        this.clients.delete(ws);
      });
    });
  }

  // Broadcast state update to all connected clients
  private broadcast(type: string, data: any): void {
    const message = JSON.stringify({ type, data });
    for (const client of this.clients) {
      if (client.readyState === WebSocket.OPEN) {
        client.send(message);
      }
    }
  }

  // Update methods called by the agent
  updateBalance(eth: string, collateral: string, symbol: string): void {
    this.state.ethBalance = eth;
    this.state.collateralBalance = collateral;
    this.state.collateralSymbol = symbol;
    this.state.lastUpdate = Date.now();
    this.broadcast("balance", { eth, collateral, symbol });
  }

  updateWallet(address: string): void {
    this.state.walletAddress = address;
    this.broadcast("wallet", { address });
  }

  updateConfig(config: Partial<DashboardState>): void {
    Object.assign(this.state, config);
    this.state.lastUpdate = Date.now();
    this.broadcast("config", config);
  }

  updateStats(stats: { received?: number; followed?: number; executed?: number; failed?: number }): void {
    if (stats.received !== undefined) this.state.signalsReceived = stats.received;
    if (stats.followed !== undefined) this.state.signalsFollowed = stats.followed;
    if (stats.executed !== undefined) this.state.tradesExecuted = stats.executed;
    if (stats.failed !== undefined) this.state.tradesFailed = stats.failed;
    this.state.lastUpdate = Date.now();
    this.broadcast("stats", this.state);
  }

  updatePositions(positions: PositionInfo[], pnl: number): void {
    this.state.positions = positions;
    this.state.activePositions = positions.length;
    this.state.totalPnL = pnl;
    this.state.lastUpdate = Date.now();
    this.broadcast("positions", { positions, pnl });
  }

  updatePlatformStats(winRate: number, totalNodes: number): void {
    this.state.platformWinRate = winRate;
    this.state.totalNodes = totalNodes;
    this.broadcast("platform", { winRate, totalNodes });
  }

  updateWsStatus(connected: boolean): void {
    this.state.wsConnected = connected;
    this.broadcast("ws_status", { connected });
  }

  addSignal(signal: ActiveSignal, nodeStats: NodeStats | null, followed: boolean, reason: string): void {
    const event: SignalEvent = {
      id: signal.id,
      tokenPair: signal.envelope.payload.token_pair,
      direction: signal.envelope.payload.direction.toUpperCase(),
      entryPrice: signal.envelope.payload.entry_price,
      nodeId: signal.envelope.node_id,
      nodeTier: nodeStats?.tier || "UNKNOWN",
      trustScore: nodeStats?.trust_score || 0,
      followed,
      reason,
      timestamp: Date.now()
    };

    this.state.recentSignals.unshift(event);
    if (this.state.recentSignals.length > 50) {
      this.state.recentSignals.pop();
    }
    this.state.signalsReceived++;
    if (followed) this.state.signalsFollowed++;
    this.state.lastUpdate = Date.now();

    this.broadcast("signal", event);
  }

  addTrade(execution: TradeExecution): void {
    const event: TradeEvent = {
      signalId: execution.signalId,
      symbol: execution.symbol,
      direction: execution.direction.toUpperCase(),
      sizeUsd: Number(execution.sizeUsd) / 1e30,
      status: execution.status,
      txHash: execution.txHash,
      error: execution.error,
      timestamp: execution.timestamp
    };

    this.state.recentTrades.unshift(event);
    if (this.state.recentTrades.length > 50) {
      this.state.recentTrades.pop();
    }

    if (execution.status === "executed") {
      this.state.tradesExecuted++;
    } else if (execution.status === "failed") {
      this.state.tradesFailed++;
    }
    this.state.lastUpdate = Date.now();

    this.broadcast("trade", event);
  }

  async start(): Promise<void> {
    return new Promise((resolve) => {
      this.server.listen(this.port, () => {
        console.log(`📊 Dashboard available at http://localhost:${this.port}`);
        resolve();
      });
    });
  }

  stop(): void {
    for (const client of this.clients) {
      client.close();
    }
    this.clients.clear();
    this.server.close();
  }
}
