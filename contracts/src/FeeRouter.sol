// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./AgentRegistry.sol";

/// @title FeeRouter
/// @notice Splits subscription payments between the agent creator and the
///         platform treasury, with the split ratio determined by the agent's
///         reputation tier.
///
///         Tier → Creator share (basis points out of 10 000):
///           BRONZE   → 6000 bps (60%)
///           SILVER   → 7000 bps (70%)
///           GOLD     → 8000 bps (80%)
///           DIAMOND  → 8500 bps (85%)
///
///         Creator earnings accumulate in pendingWithdrawals; creators call
///         withdraw() when they want to claim. Platform's share is transferred
///         immediately to the treasury wallet on each payment.
///
///         This pull-payment model for creators avoids re-entrancy and allows
///         batching many subscriptions before a single gas-efficient withdrawal.
contract FeeRouter {

    // ── Storage ───────────────────────────────────────────────────────────────

    AgentRegistry public immutable registry;
    address      public           platformTreasury;
    address      public           admin;

    // Creator fee in basis points by tier index (0=BRONZE … 3=DIAMOND).
    uint16[4] public creatorFeeBps;

    // Unclaimed creator earnings (pull-payment).
    mapping(address => uint256) public pendingWithdrawals;

    // ── Events ────────────────────────────────────────────────────────────────

    event FeesRouted(
        address indexed nodeId,
        address indexed creator,
        uint256         creatorAmount,
        uint256         platformAmount,
        uint8           tier
    );
    event Withdrawal(address indexed creator, uint256 amount);
    event TreasuryUpdated(address newTreasury);
    event CreatorFeeBpsUpdated(uint8 tier, uint16 newBps);

    // ── Errors ────────────────────────────────────────────────────────────────

    error OnlyAdmin();
    error OnlySubscriptionManager();
    error NoPendingWithdrawal();
    error InvalidBps();
    error TransferFailed();
    error NodeNotRegistered();

    // ── State: who may call route() ───────────────────────────────────────────

    address public subscriptionManager;

    // ── Constructor ───────────────────────────────────────────────────────────

    /// @param _registry         Deployed AgentRegistry address.
    /// @param _platformTreasury Platform wallet that receives its cut immediately.
    constructor(address _registry, address _platformTreasury) {
        registry         = AgentRegistry(_registry);
        platformTreasury = _platformTreasury;
        admin            = msg.sender;

        // Default fee split (matches documentation).
        creatorFeeBps[0] = 6000; // BRONZE
        creatorFeeBps[1] = 7000; // SILVER
        creatorFeeBps[2] = 8000; // GOLD
        creatorFeeBps[3] = 8500; // DIAMOND
    }

    /// @notice Called by admin after SubscriptionManager is deployed.
    function setSubscriptionManager(address _sm) external onlyAdmin {
        subscriptionManager = _sm;
    }

    // ── Core: route payment ───────────────────────────────────────────────────

    /// @notice Split an incoming payment. Only callable by SubscriptionManager.
    /// @param  nodeId Ethereum address of the signal node being subscribed to.
    /// @param  tier   Tier index (0=BRONZE … 3=DIAMOND) from ReputationOracle.
    function route(address nodeId, uint8 tier) external payable {
        if (msg.sender != subscriptionManager) revert OnlySubscriptionManager();
        if (tier > 3) revert InvalidBps();

        (,, uint256 registeredAt) = _nodeCreator(nodeId);
        if (registeredAt == 0) revert NodeNotRegistered();

        (address creator,,) = _nodeCreator(nodeId);

        uint256 creatorAmount   = (msg.value * creatorFeeBps[tier]) / 10_000;
        uint256 platformAmount  = msg.value - creatorAmount;

        // Creator share: accumulate for pull withdrawal.
        pendingWithdrawals[creator] += creatorAmount;

        // Platform share: push immediately (trusted address, no re-entrancy risk).
        (bool ok,) = platformTreasury.call{value: platformAmount}("");
        if (!ok) revert TransferFailed();

        emit FeesRouted(nodeId, creator, creatorAmount, platformAmount, tier);
    }

    // ── Creator withdrawal ────────────────────────────────────────────────────

    /// @notice Creators call this to claim their accumulated earnings.
    function withdraw() external {
        uint256 amount = pendingWithdrawals[msg.sender];
        if (amount == 0) revert NoPendingWithdrawal();

        pendingWithdrawals[msg.sender] = 0;

        (bool ok,) = msg.sender.call{value: amount}("");
        if (!ok) revert TransferFailed();

        emit Withdrawal(msg.sender, amount);
    }

    // ── Admin ─────────────────────────────────────────────────────────────────

    modifier onlyAdmin() {
        if (msg.sender != admin) revert OnlyAdmin();
        _;
    }

    function setCreatorFeeBps(uint8 tier, uint16 bps) external onlyAdmin {
        if (tier > 3 || bps > 10_000) revert InvalidBps();
        creatorFeeBps[tier] = bps;
        emit CreatorFeeBpsUpdated(tier, bps);
    }

    function setPlatformTreasury(address newTreasury) external onlyAdmin {
        platformTreasury = newTreasury;
        emit TreasuryUpdated(newTreasury);
    }

    function transferAdmin(address newAdmin) external onlyAdmin {
        admin = newAdmin;
    }

    // ── Internal ──────────────────────────────────────────────────────────────

    /// @dev The auto-generated getter for a public struct mapping returns a
    ///      tuple of individual fields, not the struct itself. Destructure explicitly.
    function _nodeCreator(address nodeId)
        internal
        view
        returns (address creator, bool active, uint256 registeredAt)
    {
        (
            ,               // name
            ,               // tokenFocus
            ,               // description
            creator,        // creator
            active,         // active
            registeredAt    // registeredAt
        ) = registry.nodes(nodeId);
    }
}
