# EPSS Fetcher

A high-performance, concurrent EPSS (Exploit Prediction Scoring System) data fetcher with support for Elasticsearch and JSON outputs.

## Features

- 🚀 **Concurrent Processing**: Configurable Stage 1 (API fetchers) and Stage 2 (bulk processors) workers
- 📊 **Multiple Output Strategies**: Elasticsearch bulk indexing or JSON file output
- 🔄 **Resume Capability**: Checkpoint system for interrupted runs
- 📈 **Progress Monitoring**: Real-time progress bar and statistics
- ⚡ **Rate Limiting**: Configurable API rate limiting with exponential backoff
- 🛡️ **Error Handling**: Comprehensive retry logic and error recovery
- 🔧 **Flexible Configuration**: YAML configuration with environment variable overrides

## Architecture

```
[EPSS API] → [Stage 1: N Fetchers] → [Direct Channel] → [Stage 2: M Processors] → [Output Strategy]
```

### Stage 1: API Fetchers
- Concurrent workers fetching data from EPSS API
- Rate limiting and retry logic
- Configurable page size (default: 100 records per request)

### Stage 2: Bulk Processors
- Short-lived workers processing batches
- Configurable bulk size (default: 2000 records)
- Strategy-based output (Elasticsearch or JSON)

## Installation

```bash
git clone https://github.com/luhtaf/epss-fetcher.git
cd epss-fetcher
go mod tidy
go build -o epss-fetcher
```

## Usage

### Basic Usage

```bash
# Run with default config
./epss-fetcher

# Run with custom config
./epss-fetcher -config config-elasticsearch.yaml

# Reset checkpoint and start fresh
./epss-fetcher -reset
```

### Configuration

Copy and customize the example configuration:

```bash
cp config.yaml.example config.yaml
```

#### Environment Variables

Sensitive configuration can be set via environment variables:

```bash
export EPSS_ELASTIC_HOSTS="https://elasticsearch:9200"
export EPSS_ELASTIC_USERNAME="elastic"
export EPSS_ELASTIC_PASSWORD="password"
export EPSS_ELASTIC_INDEX="epss-scores"
export EPSS_ELASTIC_SKIP_TLS_VERIFY="true"
export EPSS_ELASTIC_CA_CERT_PATH="/path/to/ca.crt"
export EPSS_STRATEGY="elasticsearch"
export EPSS_WORKERS_FETCHERS="10"
export EPSS_WORKERS_PROCESSORS="4"
```

### Output Strategies

#### Elasticsearch

```yaml
strategy: "elasticsearch"
elasticsearch:
  hosts:
    - "https://localhost:9200"
  index: "epss-scores"
  username: "elastic"
  password: "password"
  skip_tls_verify: true    # Skip SSL certificate verification
  ca_cert_path: ""         # Path to custom CA certificate (optional)
```

**TLS/SSL Options:**
- `skip_tls_verify: true` - For self-signed certificates or testing
- `ca_cert_path: "/path/to/ca.crt"` - For custom CA certificates
- Environment variables: `EPSS_ELASTIC_SKIP_TLS_VERIFY=true`

#### JSON Files

```yaml
strategy: "json"
json:
  output_dir: "./epss-data"
  file_pattern: "epss_batch_%04d.json"
  format: "ndjson"  # or "array"
```

## Performance Tuning

### For Elasticsearch
- Increase `bulk.size` (5000-10000) for better throughput
- Use multiple `workers.processors` if ES cluster can handle load
- Adjust `elasticsearch.timeout` based on cluster performance

### For JSON Output
- Increase `bulk.size` (10000+) for fewer files
- Use `format: "ndjson"` for better memory efficiency
- Single processor worker is usually sufficient

### API Rate Limiting
- Adjust `api.rate_limit` based on API provider limits
- Monitor for 429 responses and increase delay if needed
- Use more fetcher workers for higher throughput

## Monitoring

### Progress Bar
Real-time progress display during execution.

### Statistics File
Detailed processing statistics saved to configured file:

```
EPSS Fetcher - Processing Summary
=================================
Start Time: 2025-09-24T10:00:00Z
End Time: 2025-09-24T10:15:30Z
Duration: 15m30s
Total Records: 295356
Processed: 295000
Failed: 356
Success Rate: 99.88%
Records/sec: 318.45
```

### Checkpoint System
Automatic resume capability with checkpoint file:

```json
{
  "offset": 150000,
  "total": 295356,
  "processed": 148950,
  "last_updated": "2025-09-24T10:07:30Z",
  "start_time": "2025-09-24T10:00:00Z",
  "failed_records": []
}
```

## Docker Usage

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o epss-fetcher

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/epss-fetcher .
COPY --from=builder /app/config.yaml.example ./config.yaml
CMD ["./epss-fetcher"]
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [FIRST.org](https://www.first.org/epss/) for providing the EPSS API
- [Cyentia Institute](https://www.cyentia.com/) for EPSS research and data
