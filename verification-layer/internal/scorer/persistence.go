package scorer

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0xprotocol/verification-layer/pkg/types"
)

// AIProof contains the AI evaluation proof for a signal (for 0G Storage batch upload)
type AIProof struct {
	NodeID           string  `json:"node_id"`
	SignalID         string  `json:"signal_id"`
	Timestamp        int64   `json:"timestamp"`
	AlgorithmicScore float64 `json:"algorithmic_score"`
	AIAdjustedScore  float64 `json:"ai_adjusted_score"`
	Adjustment       float64 `json:"adjustment"`
	ManipulationFlag bool    `json:"manipulation_flag"`
	ManipulationType string  `json:"manipulation_type"`
	AIReasoning      string  `json:"ai_reasoning"`
	TEEVerified      bool    `json:"tee_verified"`
	Outcome          string  `json:"outcome"`
	PnLPercent       float64 `json:"pnl_percent"`
	FinalTier        string  `json:"final_tier"`
}

// PersistentState represents the full scorer state that gets saved to disk
type PersistentState struct {
	Version        string                            `json:"version"`
	SavedAt        int64                             `json:"saved_at"`
	NodeStats      map[string]*NodeStats             `json:"node_stats"`
	ClosedSignals  map[string][]*types.ActiveSignal  `json:"closed_signals"`
	AIProofs       []AIProof                         `json:"ai_proofs,omitempty"` // AI evaluation proofs for 0G Storage
}

const (
	stateVersion    = "1.0"
	defaultDataFile = "scorer_state.json"
)

// SaveState persists the current scorer state to disk
func (s *Scorer) SaveState(dataDir string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state := PersistentState{
		Version:       stateVersion,
		SavedAt:       time.Now().Unix(),
		NodeStats:     s.nodeStats,
		ClosedSignals: s.closedSignals,
		AIProofs:      s.aiProofs,
	}

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dataDir, defaultDataFile)
	tempPath := filePath + ".tmp"

	// Write to temp file first
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		return err
	}

	log.Printf("Scorer: State saved to %s (%d nodes, %d total signals)",
		filePath, len(state.NodeStats), countTotalSignals(state.ClosedSignals))

	return nil
}

// LoadState restores scorer state from disk
func (s *Scorer) LoadState(dataDir string) error {
	filePath := filepath.Join(dataDir, defaultDataFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Scorer: No previous state found, starting fresh")
			return nil
		}
		return err
	}

	var state PersistentState
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("Scorer: Failed to parse state file, starting fresh: %v", err)
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Restore state
	if state.NodeStats != nil {
		s.nodeStats = state.NodeStats
	}
	if state.ClosedSignals != nil {
		s.closedSignals = state.ClosedSignals
	}
	if state.AIProofs != nil {
		s.aiProofs = state.AIProofs
	}

	savedTime := time.Unix(state.SavedAt, 0)
	log.Printf("Scorer: Restored state from %s (saved %s ago, %d nodes, %d total signals)",
		filePath,
		time.Since(savedTime).Round(time.Second),
		len(s.nodeStats),
		countTotalSignals(s.closedSignals))

	return nil
}

// AutoSave starts a background goroutine that periodically saves state
func (s *Scorer) AutoSave(dataDir string, interval time.Duration, wg *sync.WaitGroup, stopCh <-chan struct{}) {
	if wg != nil {
		wg.Add(1)
	}

	go func() {
		if wg != nil {
			defer wg.Done()
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				// Final save before shutdown
				if err := s.SaveState(dataDir); err != nil {
					log.Printf("Scorer: Failed to save state on shutdown: %v", err)
				}
				return
			case <-ticker.C:
				if err := s.SaveState(dataDir); err != nil {
					log.Printf("Scorer: Auto-save failed: %v", err)
				}
			}
		}
	}()

	log.Printf("Scorer: Auto-save enabled (every %s to %s)", interval, dataDir)
}

func countTotalSignals(signals map[string][]*types.ActiveSignal) int {
	total := 0
	for _, sigs := range signals {
		total += len(sigs)
	}
	return total
}

// GetDataDir returns the default data directory for persistence
func GetDataDir() string {
	// Use current directory + /data for simplicity
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "data")
}
