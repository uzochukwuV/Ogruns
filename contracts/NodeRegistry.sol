// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title NodeRegistry
/// @notice On-chain registry of AI signal nodes. Node developers call
///         registerNode() once, then publishSignal() for every new signal blob
///         they upload to 0G Storage. The Verifier Layer watches for
///         SignalPublished events to discover and ingest new blobs.
///
/// @dev    Node identity = msg.sender = Ethereum address derived from the
///         node's private key. The same key is used to sign signal envelopes
///         off-chain (ECDSA, EIP-191) and to send on-chain transactions.
///         This means a single private key authenticates both layers without
///         any additional key management.
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

    // ── Events ────────────────────────────────────────────────────────────────

    /// @notice Emitted when a node developer registers a new node.
    event NodeRegistered(
        address indexed nodeId,
        string  name,
        string  tokenFocus,
        address creator
    );

    /// @notice Emitted when a node publishes a new signal blob to 0G Storage.
    ///         The Verifier Layer polls this event to discover new blobs.
    ///         rootHash is the 0G Storage Merkle root of the uploaded blob.
    event SignalPublished(
        address indexed nodeId,
        bytes32 rootHash,
        uint256 timestamp
    );

    /// @notice Emitted when a node is deregistered by its creator.
    event NodeDeregistered(address indexed nodeId);

    // ── Errors ────────────────────────────────────────────────────────────────

    error AlreadyRegistered();
    error NotRegistered();
    error NotActive();
    error NotCreator();

    // ── Modifiers ─────────────────────────────────────────────────────────────

    modifier onlyActiveNode() {
        if (!nodes[msg.sender].active) revert NotActive();
        _;
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

    /// @notice Publish a new signal. Called after the blob is uploaded to
    ///         0G Storage. rootHash is the storage Merkle root returned by
    ///         the 0G indexer.
    function publishSignal(bytes32 rootHash) external onlyActiveNode {
        emit SignalPublished(msg.sender, rootHash, block.timestamp);
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
