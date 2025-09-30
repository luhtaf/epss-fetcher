#!/bin/bash

# EPSS Fetcher - Import Path Fix
# This script fixes import paths for local development

echo "=== EPSS Fetcher Import Fix ==="
echo "Fixing import paths to use local modules..."

# Step 1: Update go.mod to use local module name
echo -e "\n1. Updating go.mod with local module name..."
cat > go.mod << 'EOF'
module epss-fetcher

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

# Step 2: Update all import statements in Go files
echo -e "\n2. Updating import statements in all Go files..."
find . -name "*.go" -type f -exec sed -i 's|github.com/luhtaf/epss-fetcher/|epss-fetcher/|g' {} \;

# Step 3: Clean and rebuild
echo -e "\n3. Cleaning and rebuilding..."
rm -f go.sum epss-fetcher
go mod tidy

# Step 4: Build
echo -e "\n4. Building application..."
if go build -o epss-fetcher; then
    echo -e "\n✅ SUCCESS! EPSS Fetcher built successfully!"
    echo -e "\nTo run:"
    echo -e "  ./epss-fetcher -h                    # Show help"
    echo -e "  ./epss-fetcher -config config.yaml   # Run with config"
    echo -e "\nImport paths have been fixed for local development."
else
    echo -e "\n❌ Build failed. Please check the errors above."
    exit 1
fi