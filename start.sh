#!/bin/bash

# Secure P2P File Vault - Quick Start Script
# This script helps you get started quickly

echo "🔐 Secure P2P File Vault - Quick Start"
echo "======================================"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    echo "   Visit: https://golang.org/dl/"
    exit 1
fi

echo "✅ Go $(go version | awk '{print $3}') detected"
echo ""

# Navigate to backend directory
cd backend

# Download dependencies
echo "📦 Downloading Go dependencies..."
go mod download

if [ $? -ne 0 ]; then
    echo "❌ Failed to download dependencies"
    exit 1
fi

echo "✅ Dependencies installed"
echo ""

# Start the server
echo "🚀 Starting signaling server..."
echo "   Server will be available at: http://localhost:8080"
echo ""
echo "📝 Instructions:"
echo "   1. Open http://localhost:8080 in two browser tabs"
echo "   2. In first tab: Click 'Send File' and copy the Session ID"
echo "   3. In second tab: Click 'Receive File' and paste the Session ID"
echo "   4. Select a file to transfer"
echo ""
echo "Press Ctrl+C to stop the server"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

go run main.go
