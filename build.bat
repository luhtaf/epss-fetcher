@echo off
REM EPSS Fetcher Build and Run Script for Windows
REM ==============================================

echo Building EPSS Fetcher...
go mod tidy
go build -o epss-fetcher.exe

if %errorlevel% neq 0 (
    echo Build failed!
    exit /b 1
)

echo Build completed successfully!
echo.
echo Available configurations:
echo   config.yaml.example         - General purpose configuration
echo   config-elasticsearch.yaml   - Optimized for Elasticsearch
echo   config-json.yaml            - Optimized for JSON output
echo.
echo Usage examples:
echo   epss-fetcher.exe                                    # Use config.yaml
echo   epss-fetcher.exe -config config-elasticsearch.yaml # Use Elasticsearch config
echo   epss-fetcher.exe -config config-json.yaml          # Use JSON config
echo   epss-fetcher.exe -reset                            # Reset checkpoint
echo.
echo Environment variables (optional):
echo   set EPSS_ELASTIC_HOSTS=http://localhost:9200
echo   set EPSS_ELASTIC_USERNAME=elastic
echo   set EPSS_ELASTIC_PASSWORD=password
echo   set EPSS_STRATEGY=elasticsearch