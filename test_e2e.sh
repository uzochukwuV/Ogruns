#!/bin/bash

# A simple E2E test script to verify the full flow:
# 1. The Verification Layer Backend
# 2. The AI Webhook Receiver
# 3. The Node Creator / Signal Generator

echo "=================================================="
echo "🚀 Starting 0G Verification Layer E2E Test"
echo "=================================================="

# 1. Start the Verification Layer in the background
echo "[1/3] Starting Go Verification Layer (Port 8080)..."
cd /workspace/verification-layer
go build ./cmd/verifier
./verifier > /tmp/verifier.log 2>&1 &
VERIFIER_PID=$!
sleep 3

# 2. Start the Mock AI Webhook Receiver in the background
echo "[2/3] Starting Node.js AI Webhook Receiver (Port 3000)..."
cd /workspace
node webhook_server.js > /tmp/webhook.log 2>&1 &
WEBHOOK_PID=$!
sleep 2

# 3. Run the E2E Test Script (Generates Node, Registers Webhook, Submits Signal)
echo "[3/3] Simulating Node Creator and Signal Ingestion..."
cd /workspace/verification-layer
go run ./cmd/test_e2e

# Wait for 30 seconds to let the CEX Engine process the signal and fire the Webhook
sleep 30

echo ""
echo "=================================================="
echo "🔍 Results from the AI Webhook Receiver"
echo "=================================================="
cat /tmp/webhook.log

echo ""
echo "=================================================="
echo "🏁 Cleaning up..."
echo "=================================================="
kill $VERIFIER_PID
kill $WEBHOOK_PID
echo "Done."
