package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
)

// Trading pair to CoinGecko ID mapping
// CoinGecko uses coin IDs like "bitcoin", "ethereum" instead of "BTCUSDT"
var tokenPairToCoinGeckoID = map[string]string{
	"BTCUSDT":    "bitcoin",
	"ETHUSDT":    "ethereum",
	"BNBUSDT":    "binancecoin",
	"SOLUSDT":    "solana",
	"XRPUSDT":    "ripple",
	"ADAUSDT":    "cardano",
	"DOGEUSDT":   "dogecoin",
	"DOTUSDT":    "polkadot",
	"AVAXUSDT":   "avalanche-2",
	"LINKUSDT":   "chainlink",
	"MATICUSDT":  "polygon-pos",
	"UNIUSDT":    "uniswap",
	"LTCUSDT":    "litecoin",
	"NEARUSDT":   "near",
	"ARBUSDT":    "arbitrum",
	"OPUSDT":     "optimism",
	"APTUSDT":    "aptos",
	"SUIUSDT":    "sui",
	"INJUSDT":    "injective",
	"RENDERUSDT": "render-token",
	"GRTUSDT":    "the-graph",
	"AAVEUSDT":   "aave",
	"MKRUSDT":    "maker",
	"ATOMUSDT":   "cosmos",
	"FILUSDT":    "filecoin",
	"HBARUSDT":   "hedera",
	"ICPUSDT":    "internet-computer",
	"VETUSDT":    "vechain",
	"FTMUSDT":    "fantom",
	"THETAUSDT":  "theta-network",
}

// Reverse mapping: CoinGecko ID to trading pair
var coinGeckoIDToTokenPair = make(map[string]string)

func init() {
	for pair, id := range tokenPairToCoinGeckoID {
		coinGeckoIDToTokenPair[id] = pair
	}
}

// CoinGeckoPriceFetcher polls CoinGecko API for price data ON-DEMAND
// Only fetches prices for tokens that have active signals
type CoinGeckoPriceFetcher struct {
	TickStream   chan types.Tick
	ctx          context.Context
	cancel       context.CancelFunc
	pollInterval time.Duration
	httpClient   *http.Client
	apiKey       string

	// Tracked tokens - only fetch prices for these
	mu             sync.RWMutex
	trackedTokens  map[string]int // tokenPair -> refCount (number of signals tracking this)
	lastFetchTime  time.Time
	isPolling      bool
	pollTrigger    chan struct{}
}

// CoinGeckoResponse represents the API response
type CoinGeckoResponse map[string]struct {
	USD           float64 `json:"usd"`
	USDMarketCap  float64 `json:"usd_market_cap"`
	USD24hChange  float64 `json:"usd_24h_change"`
	LastUpdatedAt int64   `json:"last_updated_at"`
}

func NewCoinGeckoPriceFetcher(tickStream chan types.Tick, apiKey string) *CoinGeckoPriceFetcher {
	ctx, cancel := context.WithCancel(context.Background())

	// Poll every 10 seconds when there are active tokens
	pollInterval := 10 * time.Second
	if apiKey != "" {
		pollInterval = 5 * time.Second
	}

	return &CoinGeckoPriceFetcher{
		TickStream:    tickStream,
		ctx:           ctx,
		cancel:        cancel,
		pollInterval:  pollInterval,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		apiKey:        apiKey,
		trackedTokens: make(map[string]int),
		pollTrigger:   make(chan struct{}, 1),
	}
}

// NewOnDemandPriceFetcher creates a price fetcher for on-demand/scheduled use
// This is simpler - no tick stream needed, just fetches prices when asked
func NewOnDemandPriceFetcher() *CoinGeckoPriceFetcher {
	ctx, cancel := context.WithCancel(context.Background())

	// Get API key from environment
	apiKey := ""
	if key := strings.TrimSpace(getEnv("COINGECKO_API_KEY", "")); key != "" {
		apiKey = key
	}

	return &CoinGeckoPriceFetcher{
		TickStream:    nil, // Not used for on-demand
		ctx:           ctx,
		cancel:        cancel,
		pollInterval:  10 * time.Second,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
		apiKey:        apiKey,
		trackedTokens: make(map[string]int),
		pollTrigger:   make(chan struct{}, 1),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func (c *CoinGeckoPriceFetcher) Start() {
	go c.pollLoop()
	log.Println("CoinGecko: On-demand price fetcher started (waiting for signals to track)")
}

func (c *CoinGeckoPriceFetcher) Stop() {
	c.cancel()
}

// TrackToken adds a token pair to be tracked. Call this when a signal is added.
func (c *CoinGeckoPriceFetcher) TrackToken(tokenPair string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we support this token
	if _, ok := tokenPairToCoinGeckoID[tokenPair]; !ok {
		log.Printf("CoinGecko: Token %s not supported, skipping", tokenPair)
		return
	}

	c.trackedTokens[tokenPair]++
	log.Printf("CoinGecko: Now tracking %s (refCount: %d, total tracked: %d)",
		tokenPair, c.trackedTokens[tokenPair], len(c.trackedTokens))

	// Trigger immediate fetch
	select {
	case c.pollTrigger <- struct{}{}:
	default:
	}
}

// UntrackToken removes a token pair from tracking. Call this when a signal closes.
func (c *CoinGeckoPriceFetcher) UntrackToken(tokenPair string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if count, ok := c.trackedTokens[tokenPair]; ok {
		if count <= 1 {
			delete(c.trackedTokens, tokenPair)
			log.Printf("CoinGecko: Stopped tracking %s (total tracked: %d)", tokenPair, len(c.trackedTokens))
		} else {
			c.trackedTokens[tokenPair]--
			log.Printf("CoinGecko: Decreased refCount for %s (refCount: %d)", tokenPair, c.trackedTokens[tokenPair])
		}
	}
}

// GetTrackedTokens returns the list of currently tracked token pairs
func (c *CoinGeckoPriceFetcher) GetTrackedTokens() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tokens := make([]string, 0, len(c.trackedTokens))
	for token := range c.trackedTokens {
		tokens = append(tokens, token)
	}
	return tokens
}

// FetchPriceOnce fetches price for a single token immediately (for signal resolution)
func (c *CoinGeckoPriceFetcher) FetchPriceOnce(tokenPair string) (float64, error) {
	// Normalize token pair: "BTC/USDT" -> "BTCUSDT"
	normalizedPair := strings.ReplaceAll(strings.ToUpper(tokenPair), "/", "")

	coinID, ok := tokenPairToCoinGeckoID[normalizedPair]
	if !ok {
		return 0, fmt.Errorf("unsupported token pair: %s (normalized: %s)", tokenPair, normalizedPair)
	}

	url := fmt.Sprintf(
		"https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd",
		coinID,
	)

	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	if c.apiKey != "" {
		req.Header.Set("x-cg-demo-api-key", c.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var prices CoinGeckoResponse
	if err := json.Unmarshal(body, &prices); err != nil {
		return 0, err
	}

	if priceData, ok := prices[coinID]; ok {
		return priceData.USD, nil
	}

	return 0, fmt.Errorf("price not found for %s", tokenPair)
}

func (c *CoinGeckoPriceFetcher) pollLoop() {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.pollTrigger:
			// Immediate fetch triggered
			c.fetchTrackedPrices()
		case <-ticker.C:
			c.fetchTrackedPrices()
		}
	}
}

func (c *CoinGeckoPriceFetcher) fetchTrackedPrices() {
	c.mu.RLock()
	trackedCount := len(c.trackedTokens)
	if trackedCount == 0 {
		c.mu.RUnlock()
		return // No tokens to track, skip fetch
	}

	// Build coin IDs for tracked tokens only
	coinIDs := make([]string, 0, trackedCount)
	for tokenPair := range c.trackedTokens {
		if coinID, ok := tokenPairToCoinGeckoID[tokenPair]; ok {
			coinIDs = append(coinIDs, coinID)
		}
	}
	c.mu.RUnlock()

	if len(coinIDs) == 0 {
		return
	}

	coinIDsStr := strings.Join(coinIDs, ",")

	url := fmt.Sprintf(
		"https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd&include_last_updated_at=true",
		coinIDsStr,
	)

	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		log.Printf("CoinGecko: Failed to create request: %v", err)
		return
	}

	if c.apiKey != "" {
		req.Header.Set("x-cg-demo-api-key", c.apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("CoinGecko: Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("CoinGecko: API error %d: %s", resp.StatusCode, string(body))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("CoinGecko: Failed to read response: %v", err)
		return
	}

	var prices CoinGeckoResponse
	if err := json.Unmarshal(body, &prices); err != nil {
		log.Printf("CoinGecko: Failed to parse response: %v", err)
		return
	}

	now := time.Now().Unix()
	for coinID, priceData := range prices {
		tokenPair, ok := coinGeckoIDToTokenPair[coinID]
		if !ok {
			continue
		}

		tick := types.Tick{
			Exchange:  "coingecko",
			TokenPair: tokenPair,
			Price:     priceData.USD,
			Timestamp: now,
		}

		select {
		case c.TickStream <- tick:
		default:
			// Channel full, skip
		}
	}

	log.Printf("CoinGecko: Fetched prices for %d tracked tokens", len(prices))
}
