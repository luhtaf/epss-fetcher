#!/bin/bash

# EPSS Fetcher - Build Diagnosis & Fix
# ====================================

echo "🔍 Diagnosing build issues..."

# Check if all required files exist
echo "📁 Checking file structure..."
required_files=(
    "main.go"
    "config/config.go" 
    "models/epss.go"
    "client/epss.go"
    "checkpoint/manager.go"
    "output/strategy.go"
    "output/elasticsearch.go"
    "output/json.go"
    "worker/fetcher.go"
    "worker/processor.go"
    "orchestrator/orchestrator.go"
    "stats/tracker.go"
)

missing_files=()
for file in "${required_files[@]}"; do
    if [ ! -f "$file" ]; then
        missing_files+=("$file")
    fi
done

if [ ${#missing_files[@]} -ne 0 ]; then
    echo "❌ Missing files:"
    printf '%s\n' "${missing_files[@]}"
    echo "Please ensure all files are present"
    exit 1
fi

echo "✅ All required files present"

# Method 1: Try building with specific Go proxy settings
echo ""
echo "🔧 Method 1: Building with local proxy settings..."
export GOPROXY=direct
export GOSUMDB=off
rm -f go.mod go.sum

go mod init github.com/luhtaf/epss-fetcher
go get github.com/schollz/progressbar/v3@v3.14.1
go get gopkg.in/yaml.v3@v3.0.1
go build -o epss-fetcher

if [ $? -eq 0 ]; then
    echo "✅ Method 1 successful!"
    ./epss-fetcher -h
    exit 0
fi

# Method 2: Manual vendor approach
echo ""
echo "🔧 Method 2: Manual dependency approach..."
rm -f go.mod go.sum

cat > go.mod << 'EOF'
module epss-fetcher

go 1.21

require (
    github.com/schollz/progressbar/v3 v3.14.1
    gopkg.in/yaml.v3 v3.0.1
)
EOF

# Update all import statements to use relative paths
echo "📝 Updating import paths..."
find . -name "*.go" -exec sed -i 's|github.com/luhtaf/epss-fetcher/|./|g' {} \;

go mod tidy
go build -o epss-fetcher

if [ $? -eq 0 ]; then
    echo "✅ Method 2 successful!"
    ./epss-fetcher -h
    exit 0
fi

# Method 3: Single-file approach (last resort)
echo ""
echo "🔧 Method 3: Checking for circular imports..."
go list -e ./...

echo ""
echo "❌ All methods failed. Manual intervention required."
echo "Please check for:"
echo "1. Circular import dependencies"
echo "2. Missing package declarations"
echo "3. Syntax errors in Go files"