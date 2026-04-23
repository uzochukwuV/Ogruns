// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./ReputationOracle.sol";
import "./FeeRouter.sol";

/// @title SubscriptionManager
/// @notice Handles subscription payments and access control.
///
///         Flow:
///           1. User calls subscribe(nodeId) with the exact payment in wei.
///           2. Contract looks up the node's current tier in ReputationOracle.
///           3. Verifies the payment matches tierPrices[tier].
///           4. Records the subscription (extending it if already active).
///           5. Immediately forwards the full payment to FeeRouter which splits
///              it between the node creator and the platform treasury.
///           6. Emits Subscribed(user, nodeId, newEndTime).
///
///         The Verifier API Gate calls isSubscribed(user, nodeId) to decide
///         whether to stream live signals to a given subscriber address.
///
///         Tier prices are set by the platform admin and denominated in the
///         native token (0G network A0GI). They can be updated as the token
///         price changes — existing subscriptions are unaffected.
contract SubscriptionManager {

    // ── Types ─────────────────────────────────────────────────────────────────

    struct Subscription {
        uint256 startTime;
        uint256 endTime;
        uint256 amountPaid; // paid for the *last* renewal
    }

    // ── Storage ───────────────────────────────────────────────────────────────

    ReputationOracle public immutable reputationOracle;
    FeeRouter        public immutable feeRouter;
    address          public admin;

    uint256 public subscriptionDuration = 30 days;

    // Tier prices in wei (A0GI on 0G Network). Admin-adjustable.
    mapping(ReputationOracle.Tier => uint256) public tierPrices;

    // user → node → Subscription
    mapping(address => mapping(address => Subscription)) public subscriptions;

    // ── Events ────────────────────────────────────────────────────────────────

    event Subscribed(
        address indexed subscriber,
        address indexed nodeId,
        uint256         endTime,
        uint256         amountPaid
    );
    event TierPriceUpdated(ReputationOracle.Tier tier, uint256 newPrice);
    event DurationUpdated(uint256 newDuration);

    // ── Errors ────────────────────────────────────────────────────────────────

    error WrongPayment(uint256 expected, uint256 sent);
    error OnlyAdmin();

    // ── Constructor ───────────────────────────────────────────────────────────

    constructor(address _reputationOracle, address _feeRouter) {
        reputationOracle = ReputationOracle(_reputationOracle);
        feeRouter        = FeeRouter(_feeRouter);
        admin            = msg.sender;

        // Default prices in A0GI wei — adjust to match market after deployment.
        // At ~$1/A0GI these are approximate $10/$25/$50/$100 equivalents.
        tierPrices[ReputationOracle.Tier.BRONZE]  = 10 ether;
        tierPrices[ReputationOracle.Tier.SILVER]  = 25 ether;
        tierPrices[ReputationOracle.Tier.GOLD]    = 50 ether;
        tierPrices[ReputationOracle.Tier.DIAMOND] = 100 ether;
    }

    // ── Core: subscribe ───────────────────────────────────────────────────────

    /// @notice Subscribe to a node. Send exactly tierPrices[node's current tier].
    ///         Calling again before expiry extends the subscription by one period.
    function subscribe(address nodeId) external payable {
        ReputationOracle.Tier tier = reputationOracle.getTier(nodeId);
        uint256 price = tierPrices[tier];

        if (msg.value != price) revert WrongPayment(price, msg.value);

        Subscription storage sub = subscriptions[msg.sender][nodeId];

        // Extend from end of current period (or from now if expired).
        uint256 base = sub.endTime > block.timestamp ? sub.endTime : block.timestamp;
        uint256 newEnd = base + subscriptionDuration;

        sub.startTime  = block.timestamp;
        sub.endTime    = newEnd;
        sub.amountPaid = msg.value;

        // Route fees: FeeRouter splits between creator and platform.
        feeRouter.route{value: msg.value}(nodeId, uint8(tier));

        emit Subscribed(msg.sender, nodeId, newEnd, msg.value);
    }

    // ── View helpers ─────────────────────────────────────────────────────────

    function isSubscribed(address user, address nodeId) external view returns (bool) {
        return subscriptions[user][nodeId].endTime > block.timestamp;
    }

    function subscriptionEnd(address user, address nodeId) external view returns (uint256) {
        return subscriptions[user][nodeId].endTime;
    }

    function priceFor(address nodeId) external view returns (uint256) {
        ReputationOracle.Tier tier = reputationOracle.getTier(nodeId);
        return tierPrices[tier];
    }

    // ── Admin ─────────────────────────────────────────────────────────────────

    modifier onlyAdmin() {
        if (msg.sender != admin) revert OnlyAdmin();
        _;
    }

    function setTierPrice(ReputationOracle.Tier tier, uint256 price) external onlyAdmin {
        tierPrices[tier] = price;
        emit TierPriceUpdated(tier, price);
    }

    function setSubscriptionDuration(uint256 durationSeconds) external onlyAdmin {
        subscriptionDuration = durationSeconds;
        emit DurationUpdated(durationSeconds);
    }

    function transferAdmin(address newAdmin) external onlyAdmin {
        admin = newAdmin;
    }
}
