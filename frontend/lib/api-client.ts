const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

export class ApiClient {
  private baseUrl: string

  constructor(baseUrl: string = API_BASE_URL) {
    this.baseUrl = baseUrl
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`

    const response = await fetch(url, {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      ...options,
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Unknown error' }))
      throw new Error(error.message || `HTTP ${response.status}`)
    }

    return response.json()
  }

  // Health check
  async healthCheck(): Promise<void> {
    await this.request('/healthz')
  }

  // Dashboard summary
  async getDashboardSummary() {
    return this.request<DashboardData>('/api/v1/analytics/dashboard')
  }

  // Node listing
  async getNodes() {
    return this.request<NodesResponse>('/api/v1/nodes')
  }

  // Node details
  async getNodeDetails(nodeId: string) {
    return this.request<NodeStats>(`/api/v1/nodes/${nodeId}`)
  }

  // Node analytics (requires authentication for premium features)
  async getNodeAnalytics(
    nodeId: string,
    params?: { from?: number; to?: number },
    auth?: { subscriberAddress: string; signature: string; timestamp: number }
  ) {
    const searchParams = new URLSearchParams()
    if (params?.from) searchParams.set('from', params.from.toString())
    if (params?.to) searchParams.set('to', params.to.toString())

    const headers: Record<string, string> = {}
    if (auth) {
      headers['X-Subscriber-Address'] = auth.subscriberAddress
      headers['X-Subscriber-Signature'] = auth.signature
      headers['X-Subscriber-Timestamp'] = auth.timestamp.toString()
    }

    const query = searchParams.toString() ? `?${searchParams.toString()}` : ''
    return this.request<NodeAnalytics>(`/api/v1/analytics/nodes/${nodeId}${query}`, {
      headers,
    })
  }

  // Portfolio simulation (requires authentication)
  async getPortfolioSimulation(
    nodeId: string,
    params: { capital?: number; from?: number; to?: number },
    auth: { subscriberAddress: string; signature: string; timestamp: number }
  ) {
    const searchParams = new URLSearchParams()
    if (params.capital) searchParams.set('capital', params.capital.toString())
    if (params.from) searchParams.set('from', params.from.toString())
    if (params.to) searchParams.set('to', params.to.toString())

    const headers = {
      'X-Subscriber-Address': auth.subscriberAddress,
      'X-Subscriber-Signature': auth.signature,
      'X-Subscriber-Timestamp': auth.timestamp.toString(),
    }

    const query = searchParams.toString() ? `?${searchParams.toString()}` : ''
    return this.request<PortfolioSimulation>(`/api/v1/analytics/nodes/${nodeId}/simulate${query}`, {
      headers,
    })
  }

  // Rate limit stats
  async getRateLimitStats() {
    return this.request<RateLimitStats>('/api/v1/stats/ratelimit')
  }

  // Submit signal (for node operators)
  async submitSignal(signalData: SignalSubmission) {
    return this.request<SignalResponse>('/api/v1/signals', {
      method: 'POST',
      body: JSON.stringify(signalData),
    })
  }

  // Register webhook (requires authentication)
  async registerWebhook(
    webhookData: WebhookRegistration,
    auth: { subscriberAddress: string; signature: string; timestamp: number }
  ) {
    const headers = {
      'X-Subscriber-Address': auth.subscriberAddress,
      'X-Subscriber-Signature': auth.signature,
      'X-Subscriber-Signature-Timestamp': auth.timestamp.toString(),
    }

    return this.request<WebhookResponse>('/api/v1/subscribers/webhook', {
      method: 'POST',
      headers,
      body: JSON.stringify(webhookData),
    })
  }
}

// Create singleton instance
export const apiClient = new ApiClient()

// Type imports (these should be in types.ts)
import type {
  DashboardData,
  NodesResponse,
  NodeStats,
  NodeAnalytics,
  PortfolioSimulation,
  RateLimitStats,
  SignalSubmission,
  SignalResponse,
  WebhookRegistration,
  WebhookResponse,
} from './types'