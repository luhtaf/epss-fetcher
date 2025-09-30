#!/bin/bash

# EPSS Fetcher - Emergency Linux Fix
# ==================================

echo "🚨 Emergency fix for Go module issues..."

# Step 1: Create a proper go.mod with replace directives
echo "📝 Creating proper go.mod..."
cat > go.mod << 'EOF'
module github.com/luhtaf/epss-fetcher

go 1.21

require (
	github.com/schollz/progressbar/v3 v3.14.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/term v0.14.0 // indirect
)
EOF

# Step 2: Download only external dependencies
echo "📥 Downloading external dependencies..."
go get github.com/schollz/progressbar/v3@v3.14.1
go get gopkg.in/yaml.v3@v3.0.1

# Step 3: Build with verbose output to debug
echo "🔨 Building with verbose output..."
go build -v -o epss-fetcher

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    echo "🎯 Testing binary..."
    ./epss-fetcher -h
    echo ""
    echo "🚀 Ready to run!"
else
    echo "❌ Build still failing. Let's try alternative approach..."
    echo ""
    echo "📋 Diagnosis:"
    echo "Current directory structure:"
    ls -la
    echo ""
    echo "Go version:"
    go version
    echo ""
    echo "Go environment:"
    go env GOPATH GOROOT GOPROXY
fi