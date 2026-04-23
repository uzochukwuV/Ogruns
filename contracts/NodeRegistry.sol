// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title NodeRegistry
/// @notice On-chain registry of AI signal nodes. Node developers call
///         registerNode() once.
///         Because of the Layer 2 Batching Architecture, individual nodes do NOT
///         publish signals here. Instead, the Verifier Layer bundles thousands of
///         signals into a daily 0G Storage blob, and calls publishBatch() to
///         anchor the proof of data on-chain.
contract NodeRegistry {

    // ── Data structures ──────────────────────────────────────────────────────

    struct NodeInfo {
        string  name;             // Human-readable display name
        string  tokenFocus;       // Primary token(s), e.g. "BTC,ETH" or "DOGE"
        string  description;      // Short description shown in marketplace
        address creator;          // Always msg.sender at registration time
        bool    active;
        uint256 registeredAt;
    }

    // ── Storage ───────────────────────────────────────────────────────────────

    mapping(address => NodeInfo) public nodes;
    address[] public nodeList;
    address public verifier;

    // ── Events ────────────────────────────────────────────────────────────────

    /// @notice Emitted when a node developer registers a new node.
    event NodeRegistered(
        address indexed nodeId,
        string  name,
        string  tokenFocus,
        address creator
    );

    /// @notice Emitted when the Verifier Layer publishes a daily batch of signals
    ///         to 0G Storage. This provides the cryptographic proof for auditing.
    event BatchPublished(
        bytes32 rootHash,
        uint256 timestamp
    );

    /// @notice Emitted when a node is deregistered by its creator.
    event NodeDeregistered(address indexed nodeId);

    // ── Errors ────────────────────────────────────────────────────────────────

    error AlreadyRegistered();
    error NotRegistered();
    error NotActive();
    error OnlyVerifier();

    // ── Modifiers ─────────────────────────────────────────────────────────────

    modifier onlyVerifier() {
        if (msg.sender != verifier) revert OnlyVerifier();
        _;
    }

    constructor(address _verifier) {
        verifier = _verifier;
    }

    // ── Core functions ────────────────────────────────────────────────────────

    /// @notice Register a new signal node. Can only be called once per address.
    function registerNode(
        string calldata name,
        string calldata tokenFocus,
        string calldata description
    ) external {
        if (nodes[msg.sender].registeredAt != 0) revert AlreadyRegistered();

        nodes[msg.sender] = NodeInfo({
            name:         name,
            tokenFocus:   tokenFocus,
            description:  description,
            creator:      msg.sender,
            active:       true,
            registeredAt: block.timestamp
        });
        nodeList.push(msg.sender);

        emit NodeRegistered(msg.sender, name, tokenFocus, msg.sender);
    }

    /// @notice Publish a daily batch of signals. Called by the Verifier after
    ///         uploading the bundled JSON to 0G Storage.
    function publishBatch(bytes32 rootHash) external onlyVerifier {
        emit BatchPublished(rootHash, block.timestamp);
    }

    /// @notice Permanently deactivate this node. Cannot be reversed.
    function deregisterNode() external {
        if (nodes[msg.sender].registeredAt == 0) revert NotRegistered();
        if (!nodes[msg.sender].active) revert NotActive();
        nodes[msg.sender].active = false;
        emit NodeDeregistered(msg.sender);
    }

    // ── View helpers ─────────────────────────────────────────────────────────

    function getNodeCount() external view returns (uint256) {
        return nodeList.length;
    }

    /// @notice Returns a paginated slice of the node list.
    function getNodes(uint256 offset, uint256 limit)
        external
        view
        returns (address[] memory)
    {
        uint256 total = nodeList.length;
        if (offset >= total) return new address[](0);
        uint256 end = offset + limit;
        if (end > total) end = total;
        address[] memory result = new address[](end - offset);
        for (uint256 i = offset; i < end; i++) {
            result[i - offset] = nodeList[i];
        }
        return result;
    }
}
