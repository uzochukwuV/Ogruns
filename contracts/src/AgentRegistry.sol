// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title IAgenticID
/// @notice Interface for 0G's ERC-7857 Agentic ID contract
interface IAgenticID {
    function ownerOf(uint256 tokenId) external view returns (address);
    function exists(uint256 tokenId) external view returns (bool);
}

/// @title AgentRegistry
/// @notice On-chain registry of AI Trading Signal Agents with optional Agentic ID verification.
///
///         This contract supports two types of agents:
///         1. Basic Agents: Registered with wallet address only (backward compatible)
///         2. Verified Agents: Linked to an ERC-7857 Agentic ID (NFT) for enhanced trust
///
///         Verified agents benefit from:
///         - Cryptographically proven AI model identity
///         - Transferable reputation (sell agent + reputation as NFT)
///         - TEE/ZKP verified capabilities
///         - Enhanced marketplace visibility
///
///         Because of the Layer 2 Batching Architecture, individual agents do NOT
///         publish signals here. Instead, the Verifier Layer bundles thousands of
///         signals into a daily 0G Storage blob, and calls publishBatch() to
///         anchor the proof of data on-chain.
contract AgentRegistry {

    // ── Data structures ──────────────────────────────────────────────────────

    struct AgentInfo {
        string  name;             // Human-readable display name
        string  tokenFocus;       // Primary token(s), e.g. "BTC,ETH" or "DOGE"
        string  description;      // Short description shown in marketplace
        address creator;          // The address that registered this agent
        bool    active;
        uint256 registeredAt;
        uint256 agenticId;        // ERC-7857 token ID (0 = not verified)
        bool    isVerified;       // True if linked to valid Agentic ID
    }

    // ── Storage ───────────────────────────────────────────────────────────────

    mapping(address => AgentInfo) public agents;
    address[] public agentList;
    address public verifier;

    // 0G Agentic ID contract (ERC-7857)
    IAgenticID public agenticIdContract;

    // Mapping from Agentic ID token to agent address (for reverse lookup)
    mapping(uint256 => address) public agenticIdToAgent;

    // ── Events ────────────────────────────────────────────────────────────────

    /// @notice Emitted when an AI agent is registered on the platform.
    event AgentRegistered(
        address indexed agentId,
        string  name,
        string  tokenFocus,
        address creator,
        uint256 agenticId,
        bool    isVerified
    );

    /// @notice Emitted when an agent links or updates their Agentic ID.
    event AgenticIdLinked(
        address indexed agentId,
        uint256 indexed agenticId
    );

    /// @notice Emitted when the Verifier Layer publishes a daily batch of signals
    ///         to 0G Storage. This provides the cryptographic proof for auditing.
    event BatchPublished(
        bytes32 rootHash,
        uint256 timestamp
    );

    /// @notice Emitted when an agent is deregistered by its creator.
    event AgentDeregistered(address indexed agentId);

    // ── Errors ────────────────────────────────────────────────────────────────

    error AlreadyRegistered();
    error NotRegistered();
    error NotActive();
    error OnlyVerifier();
    error NotAgenticIdOwner();
    error AgenticIdAlreadyLinked();
    error InvalidAgenticId();

    // ── Modifiers ─────────────────────────────────────────────────────────────

    modifier onlyVerifier() {
        if (msg.sender != verifier) revert OnlyVerifier();
        _;
    }

    constructor(address _verifier, address _agenticIdContract) {
        verifier = _verifier;
        agenticIdContract = IAgenticID(_agenticIdContract);
    }

    // ── Core functions ────────────────────────────────────────────────────────

    /// @notice Register a new AI Trading Signal Agent (basic registration).
    ///         Can optionally link an Agentic ID later via linkAgenticId().
    function registerAgent(
        string calldata name,
        string calldata tokenFocus,
        string calldata description
    ) external {
        _registerAgent(name, tokenFocus, description, 0);
    }

    /// @notice Register a new AI Trading Signal Agent with verified Agentic ID.
    ///         The caller must own the specified ERC-7857 token.
    function registerVerifiedAgent(
        string calldata name,
        string calldata tokenFocus,
        string calldata description,
        uint256 agenticId
    ) external {
        // Verify ownership of Agentic ID
        if (address(agenticIdContract) == address(0)) revert InvalidAgenticId();
        if (agenticIdContract.ownerOf(agenticId) != msg.sender) revert NotAgenticIdOwner();
        if (agenticIdToAgent[agenticId] != address(0)) revert AgenticIdAlreadyLinked();

        _registerAgent(name, tokenFocus, description, agenticId);
    }

    /// @notice Internal registration logic
    function _registerAgent(
        string calldata name,
        string calldata tokenFocus,
        string calldata description,
        uint256 agenticId
    ) internal {
        if (agents[msg.sender].registeredAt != 0) revert AlreadyRegistered();

        bool isVerified = agenticId != 0;

        agents[msg.sender] = AgentInfo({
            name:         name,
            tokenFocus:   tokenFocus,
            description:  description,
            creator:      msg.sender,
            active:       true,
            registeredAt: block.timestamp,
            agenticId:    agenticId,
            isVerified:   isVerified
        });
        agentList.push(msg.sender);

        if (isVerified) {
            agenticIdToAgent[agenticId] = msg.sender;
        }

        emit AgentRegistered(msg.sender, name, tokenFocus, msg.sender, agenticId, isVerified);
    }

    /// @notice Link an existing agent to an Agentic ID (upgrade to verified).
    ///         The caller must own both the agent and the Agentic ID token.
    function linkAgenticId(uint256 agenticId) external {
        AgentInfo storage agent = agents[msg.sender];
        if (agent.registeredAt == 0) revert NotRegistered();
        if (!agent.active) revert NotActive();
        if (agent.isVerified) revert AgenticIdAlreadyLinked();

        // Verify ownership
        if (address(agenticIdContract) == address(0)) revert InvalidAgenticId();
        if (agenticIdContract.ownerOf(agenticId) != msg.sender) revert NotAgenticIdOwner();
        if (agenticIdToAgent[agenticId] != address(0)) revert AgenticIdAlreadyLinked();

        // Link the Agentic ID
        agent.agenticId = agenticId;
        agent.isVerified = true;
        agenticIdToAgent[agenticId] = msg.sender;

        emit AgenticIdLinked(msg.sender, agenticId);
    }

    /// @notice Publish a daily batch of signals. Called by the Verifier after
    ///         uploading the bundled JSON to 0G Storage.
    function publishBatch(bytes32 rootHash) external onlyVerifier {
        emit BatchPublished(rootHash, block.timestamp);
    }

    /// @notice Permanently deactivate this agent. Cannot be reversed.
    function deregisterAgent() external {
        if (agents[msg.sender].registeredAt == 0) revert NotRegistered();
        if (!agents[msg.sender].active) revert NotActive();
        agents[msg.sender].active = false;
        emit AgentDeregistered(msg.sender);
    }

    // ── View helpers ─────────────────────────────────────────────────────────

    function getAgentCount() external view returns (uint256) {
        return agentList.length;
    }

    /// @notice Check if an agent has verified Agentic ID
    function isVerifiedAgent(address agentId) external view returns (bool) {
        return agents[agentId].isVerified;
    }

    /// @notice Get the Agentic ID token for an agent (0 if not verified)
    function getAgenticId(address agentId) external view returns (uint256) {
        return agents[agentId].agenticId;
    }

    /// @notice Get agent address by Agentic ID token
    function getAgentByAgenticId(uint256 agenticId) external view returns (address) {
        return agenticIdToAgent[agenticId];
    }

    /// @notice Returns a paginated slice of the agent list.
    function getAgents(uint256 offset, uint256 limit)
        external
        view
        returns (address[] memory)
    {
        uint256 total = agentList.length;
        if (offset >= total) return new address[](0);
        uint256 end = offset + limit;
        if (end > total) end = total;
        address[] memory result = new address[](end - offset);
        for (uint256 i = offset; i < end; i++) {
            result[i - offset] = agentList[i];
        }
        return result;
    }

    /// @notice Returns only verified agents (paginated).
    function getVerifiedAgents(uint256 offset, uint256 limit)
        external
        view
        returns (address[] memory)
    {
        // First pass: count verified agents
        uint256 verifiedCount = 0;
        for (uint256 i = 0; i < agentList.length; i++) {
            if (agents[agentList[i]].isVerified) verifiedCount++;
        }

        if (offset >= verifiedCount) return new address[](0);

        // Second pass: collect verified agents
        address[] memory verified = new address[](verifiedCount);
        uint256 idx = 0;
        for (uint256 i = 0; i < agentList.length && idx < verifiedCount; i++) {
            if (agents[agentList[i]].isVerified) {
                verified[idx++] = agentList[i];
            }
        }

        // Apply pagination
        uint256 end = offset + limit;
        if (end > verifiedCount) end = verifiedCount;
        address[] memory result = new address[](end - offset);
        for (uint256 i = offset; i < end; i++) {
            result[i - offset] = verified[i];
        }
        return result;
    }

    // ── Admin functions ──────────────────────────────────────────────────────

    /// @notice Update the Agentic ID contract address (for upgrades)
    function setAgenticIdContract(address _agenticIdContract) external onlyVerifier {
        agenticIdContract = IAgenticID(_agenticIdContract);
    }

    // ── Backward compatibility ───────────────────────────────────────────────
    // These functions maintain compatibility with existing integrations

    /// @notice Alias for registerAgent (backward compatible)
    function registerNode(
        string calldata name,
        string calldata tokenFocus,
        string calldata description
    ) external {
        _registerAgent(name, tokenFocus, description, 0);
    }

    /// @notice Alias for agents mapping (backward compatible)
    function nodes(address agentId) external view returns (
        string memory name,
        string memory tokenFocus,
        string memory description,
        address creator,
        bool active,
        uint256 registeredAt
    ) {
        AgentInfo storage a = agents[agentId];
        return (a.name, a.tokenFocus, a.description, a.creator, a.active, a.registeredAt);
    }

    /// @notice Alias for getAgentCount (backward compatible)
    function getNodeCount() external view returns (uint256) {
        return agentList.length;
    }

    /// @notice Alias for getAgents (backward compatible)
    function getNodes(uint256 offset, uint256 limit)
        external
        view
        returns (address[] memory)
    {
        uint256 total = agentList.length;
        if (offset >= total) return new address[](0);
        uint256 end = offset + limit;
        if (end > total) end = total;
        address[] memory result = new address[](end - offset);
        for (uint256 i = offset; i < end; i++) {
            result[i - offset] = agentList[i];
        }
        return result;
    }
}
