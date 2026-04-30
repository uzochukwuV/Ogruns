package aggregator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/gorilla/websocket"
)

// PriceSource defines which price feed to use
type PriceSource string

const (
	PriceSourceBinance   PriceSource = "binance"
	PriceSourceCoinGecko PriceSource = "coingecko"
	PriceSourceAuto      PriceSource = "auto" // Try Binance, fallback to CoinGecko
)

// BinanceTicker represents the JSON payload from the stream
type BinanceTicker struct {
	Stream string `json:"stream"`
	Data   struct {
		Symbol    string `json:"s"` // "BTCUSDT"
		LastPrice string `json:"c"` // "65400.50"
		EventTime int64  `json:"E"` // 1713873600000
	} `json:"data"`
}

// CEXAggregator manages the WebSocket connections to exchanges
type CEXAggregator struct {
	TickStream      chan types.Tick
	ctx             context.Context
	cancel          context.CancelFunc
	priceSource     PriceSource
	coinGeckoAPIKey string
	binanceFailed   atomic.Bool
	coinGecko       *CoinGeckoPriceFetcher
}

func NewCEXAggregator() *CEXAggregator {
	ctx, cancel := context.WithCancel(context.Background())

	// Read price source from env, default to "auto"
	source := PriceSource(os.Getenv("PRICE_SOURCE"))
	if source == "" {
		source = PriceSourceAuto
	}

	return &CEXAggregator{
		TickStream:      make(chan types.Tick, 10000), // Large buffer for high-frequency ticks
		ctx:             ctx,
		cancel:          cancel,
		priceSource:     source,
		coinGeckoAPIKey: os.Getenv("COINGECKO_API_KEY"), // Optional
	}
}

func (a *CEXAggregator) Start() {
	switch a.priceSource {
	case PriceSourceBinance:
		log.Println("Price Source: Binance WebSocket only")
		go a.streamBinance("wss://stream.binance.com:9443/ws/!miniTicker@arr")

	case PriceSourceCoinGecko:
		log.Println("Price Source: CoinGecko API only")
		a.startCoinGecko()

	case PriceSourceAuto:
		fallthrough
	default:
		log.Println("Price Source: Auto (Binance with CoinGecko fallback)")
		// Start CoinGecko immediately as backup, Binance will provide faster updates when available
		a.startCoinGecko()
		go a.streamBinance("wss://stream.binance.com:9443/ws/!miniTicker@arr")
	}
}

func (a *CEXAggregator) startCoinGecko() {
	a.coinGecko = NewCoinGeckoPriceFetcher(a.TickStream, a.coinGeckoAPIKey)
	a.coinGecko.Start()
	log.Println("CoinGecko price fetcher started")
}

func (a *CEXAggregator) Stop() {
	a.cancel()
	if a.coinGecko != nil {
		a.coinGecko.Stop()
	}
	close(a.TickStream)
}

// IsBinanceAvailable returns whether Binance WS is currently connected
func (a *CEXAggregator) IsBinanceAvailable() bool {
	return !a.binanceFailed.Load()
}

// TrackToken starts tracking a specific token pair (for on-demand price fetching)
func (a *CEXAggregator) TrackToken(tokenPair string) {
	if a.coinGecko != nil {
		a.coinGecko.TrackToken(tokenPair)
	}
}

// UntrackToken stops tracking a specific token pair
func (a *CEXAggregator) UntrackToken(tokenPair string) {
	if a.coinGecko != nil {
		a.coinGecko.UntrackToken(tokenPair)
	}
}

// FetchPriceOnce fetches the current price for a token pair immediately
func (a *CEXAggregator) FetchPriceOnce(tokenPair string) (float64, error) {
	if a.coinGecko != nil {
		return a.coinGecko.FetchPriceOnce(tokenPair)
	}
	return 0, fmt.Errorf("no price source available")
}

// GetTrackedTokens returns the list of currently tracked token pairs
func (a *CEXAggregator) GetTrackedTokens() []string {
	if a.coinGecko != nil {
		return a.coinGecko.GetTrackedTokens()
	}
	return nil
}

func (a *CEXAggregator) streamBinance(url string) {
	var conn *websocket.Conn
	var err error
	retryCount := 0
	maxRetries := 3

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			// Connect with backoff
			log.Printf("Connecting to Binance WS: %s", url)
			conn, _, err = websocket.DefaultDialer.DialContext(a.ctx, url, nil)
			if err != nil {
				retryCount++
				a.binanceFailed.Store(true)

				if retryCount >= maxRetries {
					log.Printf("Binance WS failed after %d attempts. Using CoinGecko fallback.", maxRetries)
					// Keep trying in background but less frequently
					time.Sleep(60 * time.Second)
					retryCount = 0
				} else {
					log.Printf("Binance WS Dial Error: %v. Retry %d/%d in 5s...", err, retryCount, maxRetries)
					time.Sleep(5 * time.Second)
				}
				continue
			}

			// Connected successfully
			a.binanceFailed.Store(false)
			retryCount = 0
			log.Println("Binance WS connected successfully")

			a.readLoopBinance(conn)
			conn.Close()
		}
	}
}

func (a *CEXAggregator) readLoopBinance(conn *websocket.Conn) {
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Binance WS Read Error: %v", err)
				return // Break out to trigger reconnect
			}

			var rawMessage []json.RawMessage
			if err := json.Unmarshal(message, &rawMessage); err == nil {
				// Parse array format (from stream)
				for _, raw := range rawMessage {
					var data struct {
						Symbol    string `json:"s"`
						LastPrice string `json:"c"`
						EventTime int64  `json:"E"`
					}
					if err := json.Unmarshal(raw, &data); err != nil {
						continue
					}

					price, err := strconv.ParseFloat(data.LastPrice, 64)
					if err != nil {
						continue
					}

					tokenPair := normalizeBinanceSymbol(data.Symbol)

					tick := types.Tick{
						Exchange:  "binance",
						TokenPair: tokenPair,
						Price:     price,
						Timestamp: data.EventTime / 1000,
					}

					select {
					case a.TickStream <- tick:
					default:
					}
				}
			} else {
				// Log fallback errors
				log.Printf("Binance WS Parse Error: %v", err)
			}
		}
	}
}

func normalizeBinanceSymbol(symbol string) string {
	return symbol // Leave it as BTCUSDT for simplicity
}
