// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/security/Pausable.sol";

/**
 * @title PointPayment
 * @dev Simple payment contract for purchasing points in 0G Signal Intelligence Network
 *
 * Users send ETH/tokens to this contract to purchase points.
 * Backend monitors events and credits points off-chain.
 */
contract PointPayment is Ownable, ReentrancyGuard, Pausable {

    // ═══════════════════════════════════════════════════════════════════════════
    // EVENTS
    // ═══════════════════════════════════════════════════════════════════════════

    event PointsPurchased(
        address indexed user,
        uint256 points,
        uint256 amountPaid,
        string userId,
        uint256 timestamp
    );

    event Withdrawal(
        address indexed recipient,
        uint256 amount,
        uint256 timestamp
    );

    // ═══════════════════════════════════════════════════════════════════════════
    // STATE
    // ═══════════════════════════════════════════════════════════════════════════

    uint256 public totalPayments;
    uint256 public totalPointsSold;

    mapping(address => uint256) public userPayments;

    // ═══════════════════════════════════════════════════════════════════════════
    // MAIN FUNCTION
    // ═══════════════════════════════════════════════════════════════════════════

    /**
     * @dev Purchase points by sending ETH
     * @param points Number of points to purchase
     * @param userId User ID for off-chain account linking
     */
    function purchasePoints(uint256 points, string calldata userId)
        external
        payable
        whenNotPaused
        nonReentrant
    {
        require(msg.value > 0, "Payment required");
        require(points > 0, "Invalid points amount");
        require(bytes(userId).length > 0, "User ID required");

        // Track payment
        userPayments[msg.sender] += msg.value;
        totalPayments += msg.value;
        totalPointsSold += points;

        // Emit event for backend to process
        emit PointsPurchased(
            msg.sender,
            points,
            msg.value,
            userId,
            block.timestamp
        );
    }

    /**
     * @dev Fallback function to accept direct ETH transfers
     * Note: Direct transfers won't emit PointsPurchased event
     */
    receive() external payable {
        totalPayments += msg.value;
    }

    // ═══════════════════════════════════════════════════════════════════════════
    // ADMIN FUNCTIONS
    // ═══════════════════════════════════════════════════════════════════════════

    /**
     * @dev Withdraw collected funds (owner only)
     */
    function withdraw() external onlyOwner nonReentrant {
        uint256 balance = address(this).balance;
        require(balance > 0, "No balance to withdraw");

        (bool success, ) = payable(owner()).call{value: balance}("");
        require(success, "Withdrawal failed");

        emit Withdrawal(owner(), balance, block.timestamp);
    }

    /**
     * @dev Withdraw specific amount (owner only)
     */
    function withdrawAmount(uint256 amount) external onlyOwner nonReentrant {
        require(amount > 0, "Invalid amount");
        require(address(this).balance >= amount, "Insufficient balance");

        (bool success, ) = payable(owner()).call{value: amount}("");
        require(success, "Withdrawal failed");

        emit Withdrawal(owner(), amount, block.timestamp);
    }

    /**
     * @dev Pause contract (owner only)
     */
    function pause() external onlyOwner {
        _pause();
    }

    /**
     * @dev Unpause contract (owner only)
     */
    function unpause() external onlyOwner {
        _unpause();
    }

    // ═══════════════════════════════════════════════════════════════════════════
    // VIEW FUNCTIONS
    // ═══════════════════════════════════════════════════════════════════════════

    /**
     * @dev Get contract balance
     */
    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }

    /**
     * @dev Get total payments from a user
     */
    function getUserPayments(address user) external view returns (uint256) {
        return userPayments[user];
    }

    /**
     * @dev Get contract stats
     */
    function getStats() external view returns (
        uint256 balance,
        uint256 totalPaymentsReceived,
        uint256 totalPoints
    ) {
        return (
            address(this).balance,
            totalPayments,
            totalPointsSold
        );
    }
}
