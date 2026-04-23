package aggregator

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
	"github.com/gorilla/websocket"
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
	TickStream chan types.Tick
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewCEXAggregator() *CEXAggregator {
	ctx, cancel := context.WithCancel(context.Background())
	return &CEXAggregator{
		TickStream: make(chan types.Tick, 10000), // Large buffer for high-frequency ticks
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (a *CEXAggregator) Start() {
	// For MVP, we stream the Binance all-market mini-ticker
	// In production, you would subscribe only to active TokenPairs to save bandwidth
	go a.streamBinance("wss://stream.binance.com:9443/ws/!miniTicker@arr")
}

func (a *CEXAggregator) Stop() {
	a.cancel()
	close(a.TickStream)
}

func (a *CEXAggregator) streamBinance(url string) {
	var conn *websocket.Conn
	var err error

	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			// Connect with backoff
			log.Printf("Connecting to Binance WS: %s", url)
			conn, _, err = websocket.DefaultDialer.DialContext(a.ctx, url, nil)
			if err != nil {
				log.Printf("Binance WS Dial Error: %v. Retrying in 5s...", err)
				time.Sleep(5 * time.Second)
				continue
			}

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
	if strings.HasSuffix(symbol, "USDT") {
		return strings.TrimSuffix(symbol, "USDT") + "/USDT"
	}
	return symbol // Fallback
}
