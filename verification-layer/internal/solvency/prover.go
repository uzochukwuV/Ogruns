package solvency

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Database interface for solvency operations.
type Database interface {
	GetAllPointBalances(ctx context.Context) (map[string]int64, error)
	SumAllPoints(ctx context.Context) (int64, error)
	RecordSolvencySnapshot(ctx context.Context, merkleRoot string, totalUserPoints, totalProviderPoints int64, totalOutstandingOG, vaultBalanceOG *big.Float, isSolvent bool, publishedTxHash, storageRootHash string) error
}

// VaultContract interface for on-chain operations.
type VaultContract interface {
	GetBalance(ctx context.Context) (*big.Int, error)
	PublishSolvencyProof(ctx context.Context, merkleRoot [32]byte, totalPoints *big.Int, totalValueLocked *big.Int) (string, error)
}

// StorageClient interface for 0G Storage operations.
type StorageClient interface {
	UploadJSON(ctx context.Context, data interface{}) (string, error)
}

// Prover generates and publishes solvency proofs.
type Prover struct {
	db            Database
	vaultContract VaultContract
	storageClient StorageClient // Optional 0G Storage client
	interval      time.Duration
	stopChan      chan struct{}
	doneChan      chan struct{}
}

// Config holds configuration for the solvency prover.
type Config struct {
	IntervalHours int // Hours between solvency proofs (default: 6)
}

// NewProver creates a new solvency prover.
func NewProver(db Database, vaultContract VaultContract, storageClient StorageClient, cfg *Config) *Prover {
	interval := 6 * time.Hour
	if cfg != nil && cfg.IntervalHours > 0 {
		interval = time.Duration(cfg.IntervalHours) * time.Hour
	}

	return &Prover{
		db:            db,
		vaultContract: vaultContract,
		storageClient: storageClient,
		interval:      interval,
		stopChan:      make(chan struct{}),
		doneChan:      make(chan struct{}),
	}
}

// Start begins the periodic solvency proof publishing loop.
func (p *Prover) Start(ctx context.Context) {
	log.Printf("[SolvencyProver] Starting solvency proof publisher (interval: %v)...", p.interval)
	go p.proofLoop(ctx)
}

// Stop gracefully stops the prover.
func (p *Prover) Stop() {
	log.Println("[SolvencyProver] Stopping solvency proof publisher...")
	close(p.stopChan)
	<-p.doneChan
	log.Println("[SolvencyProver] Solvency proof publisher stopped.")
}

// proofLoop is the main publishing loop.
func (p *Prover) proofLoop(ctx context.Context) {
	defer close(p.doneChan)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run immediately on startup
	if err := p.publishProof(ctx); err != nil {
		log.Printf("[SolvencyProver] Initial proof failed: %v", err)
	}

	for {
		select {
		case <-p.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.publishProof(ctx); err != nil {
				log.Printf("[SolvencyProver] Proof publishing failed: %v", err)
			}
		}
	}
}

// publishProof generates and publishes a solvency proof.
func (p *Prover) publishProof(ctx context.Context) error {
	log.Println("[SolvencyProver] ──────────────────────────────────────────────")
	log.Println("[SolvencyProver] Generating solvency proof...")

	// Step 1: Fetch all point balances from DB
	balances, err := p.db.GetAllPointBalances(ctx)
	if err != nil {
		return fmt.Errorf("failed to get point balances: %w", err)
	}

	if len(balances) == 0 {
		log.Println("[SolvencyProver] No balances to prove, skipping.")
		return nil
	}

	log.Printf("[SolvencyProver] Fetched %d account balances", len(balances))

	// Step 2: Build Merkle tree
	tree, totalPoints := buildMerkleTree(balances)
	merkleRoot := tree.Root

	log.Printf("[SolvencyProver] Built Merkle tree with %d leaves", len(tree.Leaves))
	log.Printf("[SolvencyProver] Merkle root: %s", common.BytesToHash(merkleRoot[:]).Hex())
	log.Printf("[SolvencyProver] Total outstanding points: %d", totalPoints)

	// Step 3: Calculate total value locked in 0G (points / 1000)
	totalValueLockedFloat := new(big.Float).SetInt64(totalPoints)
	totalValueLockedFloat.Quo(totalValueLockedFloat, big.NewFloat(1000.0))

	// Convert to wei
	oneEther := new(big.Float).SetInt(big.NewInt(1e18))
	totalValueLockedWeiFloat := new(big.Float).Mul(totalValueLockedFloat, oneEther)
	totalValueLockedWei, _ := totalValueLockedWeiFloat.Int(nil)

	log.Printf("[SolvencyProver] Total value locked: %.6f 0G", floatFromWei(totalValueLockedWei))

	// Step 4: Query vault balance on-chain
	vaultBalance, err := p.vaultContract.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("failed to get vault balance: %w", err)
	}

	log.Printf("[SolvencyProver] Vault balance: %.6f 0G", floatFromWei(vaultBalance))

	// Step 5: Check solvency
	isSolvent := vaultBalance.Cmp(totalValueLockedWei) >= 0
	shortfall := new(big.Int).Sub(totalValueLockedWei, vaultBalance)

	if isSolvent {
		log.Printf("[SolvencyProver] ✅ SOLVENT - Vault has sufficient funds")
	} else {
		log.Printf("[SolvencyProver] ⚠️  WARNING: INSOLVENT - Shortfall: %.6f 0G", floatFromWei(shortfall))
		log.Printf("[SolvencyProver] ⚠️  This is informational only. Manual intervention may be required.")
	}

	// Step 6: Upload full Merkle tree to 0G Storage (optional)
	var storageRootHash string
	if p.storageClient != nil {
		storageRootHash, err = p.storageClient.UploadJSON(ctx, tree)
		if err != nil {
			log.Printf("[SolvencyProver] Failed to upload Merkle tree to 0G Storage: %v", err)
			// Continue anyway - storage upload is optional
		} else {
			log.Printf("[SolvencyProver] Uploaded Merkle tree to 0G Storage: %s", storageRootHash)
		}
	}

	// Step 7: Publish Merkle root on-chain
	txHash, err := p.vaultContract.PublishSolvencyProof(ctx, merkleRoot, big.NewInt(totalPoints), totalValueLockedWei)
	if err != nil {
		return fmt.Errorf("failed to publish solvency proof on-chain: %w", err)
	}

	log.Printf("[SolvencyProver] Published solvency proof on-chain (tx=%s)", txHash)

	// Step 8: Record snapshot in DB
	vaultBalanceFloat := new(big.Float).SetInt(vaultBalance)
	vaultBalanceFloat.Quo(vaultBalanceFloat, oneEther)

	if err := p.db.RecordSolvencySnapshot(ctx, common.BytesToHash(merkleRoot[:]).Hex(), 0, totalPoints, totalValueLockedFloat, vaultBalanceFloat, isSolvent, txHash, storageRootHash); err != nil {
		log.Printf("[SolvencyProver] Failed to record snapshot in DB: %v", err)
		// Continue anyway - the on-chain proof is the source of truth
	}

	log.Println("[SolvencyProver] ✅ Solvency proof cycle complete")
	log.Println("[SolvencyProver] ──────────────────────────────────────────────")

	return nil
}

// ─── Merkle Tree Implementation ───────────────────────────────────────────────

// MerkleTree represents a binary Merkle tree for solvency proofs.
type MerkleTree struct {
	Root   [32]byte        `json:"root"`
	Leaves []MerkleLeaf    `json:"leaves"`
	Proofs map[string][]string `json:"proofs"` // address -> proof path (hex hashes)
}

// MerkleLeaf represents a single account balance leaf.
type MerkleLeaf struct {
	Address string `json:"address"`
	Points  int64  `json:"points"`
	Hash    string `json:"hash"`
}

// buildMerkleTree builds a Merkle tree from account balances.
// Returns the tree and total points.
func buildMerkleTree(balances map[string]int64) (*MerkleTree, int64) {
	// Sort addresses for deterministic tree construction
	addresses := make([]string, 0, len(balances))
	for addr := range balances {
		addresses = append(addresses, addr)
	}
	sort.Strings(addresses)

	// Build leaves: hash = keccak256(address || uint256(points))
	leaves := make([]MerkleLeaf, len(addresses))
	hashes := make([][32]byte, len(addresses))
	totalPoints := int64(0)

	for i, addr := range addresses {
		points := balances[addr]
		totalPoints += points

		// Create leaf hash: keccak256(address || points)
		leafData := make([]byte, 52) // 20 bytes address + 32 bytes points
		copy(leafData[0:20], common.HexToAddress(addr).Bytes())
		pointsBig := big.NewInt(points)
		pointsBytes := pointsBig.Bytes()
		copy(leafData[52-len(pointsBytes):], pointsBytes) // Right-pad to 32 bytes

		leafHash := crypto.Keccak256Hash(leafData)
		hashes[i] = leafHash

		leaves[i] = MerkleLeaf{
			Address: addr,
			Points:  points,
			Hash:    leafHash.Hex(),
		}
	}

	// Build tree bottom-up
	currentLevel := hashes
	proofPaths := make(map[int][][32]byte) // leaf index -> proof path

	for i := range leaves {
		proofPaths[i] = [][32]byte{}
	}

	for len(currentLevel) > 1 {
		nextLevel := make([][32]byte, 0)

		for i := 0; i < len(currentLevel); i += 2 {
			var parentHash [32]byte

			if i+1 < len(currentLevel) {
				// Pair exists - hash both
				left := currentLevel[i]
				right := currentLevel[i+1]
				parentHash = hashPair(left, right)

				// Add sibling to proof paths
				for j := 0; j < len(leaves); j++ {
					if proofPaths[j] != nil {
						// If this leaf is in the left subtree, add right as proof
						// If in right subtree, add left as proof
						// (simplified - in production you'd track which subtree each leaf is in)
					}
				}
			} else {
				// Odd node - promote to next level
				parentHash = currentLevel[i]
			}

			nextLevel = append(nextLevel, parentHash)
		}

		currentLevel = nextLevel
	}

	root := currentLevel[0]

	// Build proof map (simplified - full implementation would track proof paths during tree construction)
	proofs := make(map[string][]string)
	for _, leaf := range leaves {
		// TODO: In production, compute actual Merkle proof path for each leaf
		proofs[leaf.Address] = []string{leaf.Hash}
	}

	return &MerkleTree{
		Root:   root,
		Leaves: leaves,
		Proofs: proofs,
	}, totalPoints
}

// hashPair hashes two nodes to create a parent node.
func hashPair(left, right [32]byte) [32]byte {
	// Sort hashes to make tree construction deterministic
	if string(left[:]) > string(right[:]) {
		left, right = right, left
	}

	data := make([]byte, 64)
	copy(data[0:32], left[:])
	copy(data[32:64], right[:])

	return crypto.Keccak256Hash(data)
}

// floatFromWei converts wei to 0G as a float64.
func floatFromWei(wei *big.Int) float64 {
	weiFloat := new(big.Float).SetInt(wei)
	oneEther := new(big.Float).SetInt(big.NewInt(1e18))
	ogFloat := new(big.Float).Quo(weiFloat, oneEther)
	og, _ := ogFloat.Float64()
	return og
}
