import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";

// Load environment variables from .env in the contracts directory.
// Required: DEPLOYER_PRIVATE_KEY, (optional) ETHERSCAN_API_KEY
const dotenv = require("dotenv");
dotenv.config();

const deployerKey = process.env.DEPLOYER_PRIVATE_KEY || "0x" + "0".repeat(64);

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.24",
    settings: {
      optimizer: { enabled: true, runs: 200 },
    },
  },

  networks: {
    // 0G Galileo Testnet
    zg_testnet: {
      url: "https://evmrpc-testnet.0g.ai",
      chainId: 16602,
      accounts: [deployerKey],
      gasPrice: "auto",
    },
    // 0G Mainnet (update when available)
    zg_mainnet: {
      url: process.env.ZG_MAINNET_RPC || "https://evmrpc.0g.ai",
      chainId: 16661, // update when mainnet chain ID is published
      accounts: [deployerKey],
    },
    // Local Hardhat node for unit tests
    hardhat: {
      chainId: 31337,
    },
  },

  // Optional: configure Etherscan-compatible explorer for contract verification
  etherscan: {
    apiKey: process.env.ETHERSCAN_API_KEY || "UD6XAYNRKSMX3ASYKG9FTH87XKEY4DBVCY",
    customChains: [
      {
        network: "zg_testnet",
        chainId: 16602,
        urls: {
          apiURL: "https://chainscan-galileo.0g.ai/api",
          browserURL: "https://chainscan-galileo.0g.ai",
        },
      },
    ],
  },

  paths: {
    sources: "./src",
    tests: "./test",
    cache: "./cache",
    artifacts: "./artifacts",
  },
};

export default config;
