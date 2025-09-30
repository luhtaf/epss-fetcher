#!/bin/bash

# EPSS Fetcher - Linux Setup Script
# ==================================

echo "🚀 Setting up EPSS Fetcher on Linux..."

# Step 1: Initialize Go module properly
echo "📦 Initializing Go module..."
rm -f go.mod go.sum 2>/dev/null || true
go mod init github.com/luhtaf/epss-fetcher

# Step 2: Add dependencies
echo "📥 Adding dependencies..."
go mod tidy

# Step 3: Build application
echo "🔨 Building application..."
go build -o epss-fetcher

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo ""
    echo "🎯 Quick start commands:"
    echo "  ./epss-fetcher -h                           # Show help"
    echo "  ./epss-fetcher -config config-json.yaml    # JSON output (recommended)"
    echo "  ./epss-fetcher -date 2025-09-29            # Incremental mode"
    echo "  ./epss-fetcher -incremental                # Auto-incremental"
    echo ""
    echo "📁 Configuration files:"
    echo "  config-json.yaml         - JSON output strategy"
    echo "  config-elasticsearch.yaml - Elasticsearch strategy"
    echo "  config.yaml.example      - Template configuration"
    echo ""
    echo "🚀 Ready to run!"
else
    echo "❌ Build failed. Checking dependencies..."
    go mod tidy
    echo "🔄 Retrying build..."
    go build -o epss-fetcher
fi