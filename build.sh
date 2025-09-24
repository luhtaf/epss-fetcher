#!/bin/bash

# EPSS Fetcher Build and Run Script
# ==================================

set -e

echo "Building EPSS Fetcher..."
go mod tidy
go build -o epss-fetcher

echo "Build completed successfully!"
echo ""
echo "Available configurations:"
echo "  config.yaml.example         - General purpose configuration"
echo "  config-elasticsearch.yaml   - Optimized for Elasticsearch"
echo "  config-json.yaml            - Optimized for JSON output"
echo ""
echo "Usage examples:"
echo "  ./epss-fetcher                                    # Use config.yaml"
echo "  ./epss-fetcher -config config-elasticsearch.yaml # Use Elasticsearch config"
echo "  ./epss-fetcher -config config-json.yaml          # Use JSON config"
echo "  ./epss-fetcher -reset                            # Reset checkpoint"
echo ""
echo "Environment variables (optional):"
echo "  export EPSS_ELASTIC_HOSTS=\"http://localhost:9200\""
echo "  export EPSS_ELASTIC_USERNAME=\"elastic\""
echo "  export EPSS_ELASTIC_PASSWORD=\"password\""
echo "  export EPSS_STRATEGY=\"elasticsearch\""