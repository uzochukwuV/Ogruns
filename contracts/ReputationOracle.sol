// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title ReputationOracle
/// @notice The single source of truth for Node Trust Scores on 0G Network.
///
///         Only the authorised Verifier address (a platform-controlled key or
///         multisig) can write scores. Every other contract reads from here to
///         determine a node's tier and therefore its subscription price and
///         creator fee split.
///
/// @dev    Scores are stored as uint256 with 2 decimal places of precision
///         (e.g. 7542 = 75.42 / 100). This avoids floating-point and keeps
///         gas costs low (single SSTORE per update).
///
///         Trust Score → Tier mapping (same as the Go scorer):
///           0    – 4000   →  BRONZE   (score 0–40)
///           4001 – 7000   →  SILVER   (score 41–70)
///           7001 – 9000   →  GOLD     (score 71–90)
///           9001 – 10000  →  DIAMOND  (score 91–100)
contract ReputationOracle {

    // ── Types ─────────────────────────────────────────────────────────────────

    enum Tier { BRONZE, SILVER, GOLD, DIAMOND }

    struct NodeScore {
        uint256 score;      // 0–10000 (2 decimal places)
        Tier    tier;
        uint256 updatedAt;  // block.timestamp of last update
    }

    // ── Storage ───────────────────────────────────────────────────────────────

    address public verifier;     // Only this address may call updateScore
    address public pendingVerifier;

    mapping(address => NodeScore) public scores;

    // ── Events ────────────────────────────────────────────────────────────────

    event ScoreUpdated(
        address indexed nodeId,
        uint256 score,
        Tier    tier,
        uint256 timestamp
    );
    event TierChanged(
        address indexed nodeId,
        Tier    oldTier,
        Tier    newTier
    );
    event VerifierTransferProposed(address indexed proposed);
    event VerifierTransferred(address indexed oldVerifier, address indexed newVerifier);

    // ── Errors ────────────────────────────────────────────────────────────────

    error OnlyVerifier();
    error OnlyPendingVerifier();
    error ScoreOutOfRange();

    // ── Constructor ───────────────────────────────────────────────────────────

    constructor(address _verifier) {
        verifier = _verifier;
    }

    // ── Core: write score (verifier only) ────────────────────────────────────

    /// @notice Update a node's trust score and tier. Called by the Broadcaster.
    /// @param  nodeId  Ethereum address = node identity.
    /// @param  score   Trust score × 100 (e.g. 7542 for 75.42).
    /// @param  tier    Tier enum value (0=BRONZE … 3=DIAMOND).
    function updateScore(address nodeId, uint256 score, uint8 tier) external {
        if (msg.sender != verifier) revert OnlyVerifier();
        if (score > 10000)          revert ScoreOutOfRange();
        if (tier > uint8(Tier.DIAMOND)) revert ScoreOutOfRange();

        Tier newTier = Tier(tier);
        Tier oldTier = scores[nodeId].tier;

        scores[nodeId] = NodeScore({
            score:     score,
            tier:      newTier,
            updatedAt: block.timestamp
        });

        emit ScoreUpdated(nodeId, score, newTier, block.timestamp);

        if (newTier != oldTier) {
            emit TierChanged(nodeId, oldTier, newTier);
        }
    }

    // ── View helpers ─────────────────────────────────────────────────────────

    function getScore(address nodeId)
        external view
        returns (uint256 score, Tier tier, uint256 updatedAt)
    {
        NodeScore memory s = scores[nodeId];
        return (s.score, s.tier, s.updatedAt);
    }

    function getTier(address nodeId) external view returns (Tier) {
        return scores[nodeId].tier;
    }

    // ── Verifier transfer (two-step) ─────────────────────────────────────────

    /// @notice Propose a new verifier address (step 1).
    function proposeVerifier(address proposed) external {
        if (msg.sender != verifier) revert OnlyVerifier();
        pendingVerifier = proposed;
        emit VerifierTransferProposed(proposed);
    }

    /// @notice New verifier accepts ownership (step 2).
    function acceptVerifier() external {
        if (msg.sender != pendingVerifier) revert OnlyPendingVerifier();
        emit VerifierTransferred(verifier, pendingVerifier);
        verifier = pendingVerifier;
        pendingVerifier = address(0);
    }
}
