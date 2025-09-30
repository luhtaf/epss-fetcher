# EPSS Fetcher - Linux Troubleshooting Guide

## � CRITICAL: "no matching versions" Error Fix

### Issue: Go Module Import Resolution Failure
```bash
go: github.com/luhtaf/epss-fetcher/output: no matching versions for query "latest"
```

**Root Cause:** Go is trying to download local packages from remote repository.

## 🔧 **SOLUTION 1: Use Emergency Fix Script**
```bash
chmod +x fix-linux.sh
./fix-linux.sh
```

## 🔧 **SOLUTION 2: Use Build Diagnosis Script**
```bash
chmod +x diagnose-build.sh
./diagnose-build.sh
```

## 🔧 **SOLUTION 3: Manual Step-by-Step Fix**

### Step 1: Set Go Proxy to Direct
```bash
export GOPROXY=direct
export GOSUMDB=off
```

### Step 2: Clean and Reinitialize with Simple Module Name
```bash
rm -f go.mod go.sum
go mod init epss-fetcher  # Use simple name, not GitHub path
```

### Step 3: Add External Dependencies Only
```bash
go get github.com/schollz/progressbar/v3@v3.14.1
go get gopkg.in/yaml.v3@v3.0.1
```

### Step 4: Fix Import Paths in Source Files
```bash
# Replace all GitHub import paths with relative paths
find . -name "*.go" -exec sed -i 's|github.com/luhtaf/epss-fetcher/|./|g' {} \;
```

### Step 5: Build
```bash
go build -o epss-fetcher
```

### Issue 2: Import Path Problems
**Root Cause:** Local package imports require proper module initialization.

**Solution:** Use the setup script:
```bash
chmod +x setup-linux.sh
./setup-linux.sh
```

### Issue 3: Permission Denied
```bash
chmod +x epss-fetcher
chmod +x setup-linux.sh
chmod +x build.sh
```

### Issue 4: Missing Dependencies
```bash
# Ensure Go 1.21+ is installed
go version

# Install dependencies manually if needed
go get github.com/schollz/progressbar/v3@v3.14.1
go get gopkg.in/yaml.v3@v3.0.1
```

## 🚀 Quick Setup Commands

### One-liner Setup:
```bash
rm -f go.mod go.sum && go mod init github.com/luhtaf/epss-fetcher && go mod tidy && go build -o epss-fetcher
```

### Verify Setup:
```bash
./epss-fetcher -h
```

### First Run:
```bash
# Small test (recommended first)
./epss-fetcher -config config.yaml

# Production run  
./epss-fetcher -config config-json.yaml

# Incremental mode
./epss-fetcher -date 2025-09-29 -config config-json.yaml
```

## 📁 Expected File Structure
```
epss-fetcher/
├── epss-fetcher           # Binary (created after build)
├── go.mod                 # Go module file
├── go.sum                 # Dependency checksums
├── main.go               # Entry point
├── config/               # Configuration package
├── client/               # EPSS API client
├── models/               # Data models
├── worker/               # Processing workers
├── output/               # Output strategies
├── orchestrator/         # Main orchestrator
├── checkpoint/           # Checkpoint management
├── stats/                # Statistics tracking
└── config-*.yaml         # Configuration files
```

## 🔧 Manual Build Steps
If setup script fails:

```bash
# 1. Check Go version
go version  # Should be 1.21+

# 2. Clean workspace  
rm -f go.mod go.sum epss-fetcher

# 3. Initialize module
go mod init github.com/luhtaf/epss-fetcher

# 4. Download dependencies
go mod tidy

# 5. Build binary
go build -o epss-fetcher

# 6. Set permissions
chmod +x epss-fetcher

# 7. Test
./epss-fetcher -h
```

## ✅ Success Indicators
- `go mod tidy` completes without errors
- `go build -o epss-fetcher` creates binary successfully  
- `./epss-fetcher -h` shows help message
- No import path errors in logs

## 🎯 Ready to Run
Once build succeeds:
```bash
./epss-fetcher -config config-json.yaml
```