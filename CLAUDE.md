# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

prometheus-nats-exporter is a Prometheus exporter for NATS server monitoring. It aggregates metrics from NATS server HTTP monitoring endpoints (varz, connz, subz, routez, healthz, jsz, etc.) and exposes them in Prometheus format.

### Place in the ecosystem

- **Sibling project**: `nats-io/nats-surveyor` monitors NATS too, but via the NATS system account (`$SYS.REQ` messages, JSAPI advisories). This exporter is HTTP-poll only. Do **not** propose surveyor-style features (system account auth, advisory subscriptions) here — they belong in surveyor.
- **Deployment model**: One exporter sidecar per nats-server. Wired into `nats-io/k8s` Helm charts as a StatefulSet sidecar.
- **Grafana dashboards**: Pre-built JSON in `walkthrough/` (both `gnatsd`-prefixed and Helm `nats`-prefixed variants).

## Build and Test Commands

### Building
```bash
make build          # Build the binary
```

### Testing
```bash
make test           # Run all tests with race detection
make test-cov       # Run tests with coverage output (generates collector.out and exporter.out)

# Run tests for specific packages
go test -v -race -count=1 -parallel=1 ./test/...
go test -v -race -count=1 -parallel=1 ./collector/...
go test -v -race -count=1 -parallel=1 ./exporter/...
```

### Linting
```bash
make lint           # Run go vet and golangci-lint
```

Note: Linting requires golangci-lint to be installed. See `.golangci.yaml` for enabled linters.

### Formatting
Always format Go code with:
```bash
go fmt ./...
```

### Docker
```bash
make dockerx        # Build docker image using buildx
```

### Running the Exporter
```bash
# Basic usage - monitor varz endpoint
./prometheus-nats-exporter -varz "http://localhost:8222"

# With multiple endpoints
./prometheus-nats-exporter -varz -connz -routez "http://localhost:8222"

# Default metrics endpoint
# http://0.0.0.0:7777/metrics
```

If no metric flags are passed, the exporter defaults to `-varz` only (not "nothing"). See `main.go` flag handling.

## Architecture

### Package Structure

**`main.go`** - Entry point
- Parses command-line flags
- Creates exporter instance
- Manages graceful shutdown
- Supports multiple NATS servers (though not recommended per Prometheus best practices)
- Handles server ID parsing: can use URL-based ID, custom ID, or fetch from /varz endpoint

**`exporter/`** - HTTP server and coordination
- `NATSExporter`: Main coordinator that manages HTTP server and collectors
- Registers Prometheus collectors for each enabled metric type
- Handles basic auth for scrape endpoint if configured
- Supports TLS configuration
- Default listen: `0.0.0.0:7777/metrics`

**`collector/`** - Metric collection logic
- Each NATS endpoint has its own collector file (e.g., `varz.go`, `connz.go`, `jsz.go`)
- `NATSCollector`: Base type for standard collectors using reflection-based metric extraction (`objectToMetrics` in `collector.go`)
- JetStream collector (`jsz.go`): Custom implementation using Prometheus `Describe`/`Collect` pattern — required because nested account→stream→consumer hierarchy needs typed labels with fixed cardinality
- Custom (non-reflection) collectors: `connz.go`, `gatewayz.go`, `leafz.go`, `accountz.go`, `accstatz.go`, `healthz.go`, `jsz.go`. Reflection-only: `varz.go`, `subz.go`, `routez.go`
- Collectors poll NATS HTTP endpoints and transform JSON responses into Prometheus metrics
- All metrics are gauges (snapshots of current state)
- Metric naming: Uses "gnatsd" namespace for backward compatibility (not "nats")
- **Reflection skip-list** (`collector.go:338-348`): 8 top-level JSON fields are explicitly dropped (`cluster_*`, `gateway_*`, `trusted_operators_claim`). Adding a new field to varz/etc. does NOT automatically expose it if it matches one of these prefixes
- **`healthz.go` dual-metric pattern**: emits both numeric `status` (0=ok, 1=error) and `statusValue` with a `value` label (`ok|unreachable|error`). Both are intentional — preserve when modifying
- **`mapKeys()` schema drift**: collector tracks response keys across polls; if NATS server changes its JSON shape, collectors are re-initialized rather than silently dropping new fields

**`test/`** - Testing utilities
- Provides helpers to run embedded NATS servers for testing
- Default ports: ClientPort=11224, MonitorPort=11424, StaticPort=11425
- **Static fixture servers** for endpoints not easily exercised from an embedded NATS: `RunJszStaticServer`, `RunGatewayzStaticServer`, `RunAccstatzStaticServer`, `RunLeafzStaticServer`. New collector tests for these endpoints should use the static-server pattern, not try to coerce embedded NATS into producing the shape

### Key Architectural Patterns

1. **Dual API**: Can be used as standalone binary OR embedded as a Go library
2. **Collector Registration**: Each metric type (varz, connz, etc.) registers as a Prometheus collector
3. **Server ID Handling**: Three modes:
   - URL-based (default): Uses scheme://host as ID
   - Custom: Format `id,url`
   - Internal: Fetches ServerID or ServerName from /varz endpoint
4. **Retry Logic**: If NATS server is unavailable, retries with configurable interval (default 30s)
5. **Logging**: Supports console, file, syslog, and remote syslog

### Metric Collection Flow

1. Exporter creates HTTP server on specified port
2. For each enabled metric type, creates corresponding collector
3. Collectors are registered with Prometheus registry
4. On scrape request to `/metrics`:
   - Prometheus calls `Collect()` on each registered collector
   - Collector makes HTTP request to NATS monitoring endpoint
   - JSON response is parsed and converted to Prometheus metrics
   - Metrics are returned to Prometheus

### JetStream Metrics

JetStream metrics (`-jsz` flag) support filtering:
- `streams`: Stream-level metrics only
- `accounts`: Account-level metrics only
- `consumers`: Consumer-level metrics only
- Can combine: `-jsz "streams,consumers"`
- `-jsz` value is a string filter, not a boolean. Empty string disables JetStream collection

**Custom label injection from stream/consumer metadata**:
- `-jsz_stream_meta_keys "key1,key2"` — promote stream metadata entries to Prometheus labels
- `-jsz_consumer_meta_keys "key1,key2"` — same for consumer metadata
- Key names are regex-validated (`[a-zA-Z0-9_]+`); invalid entries fail startup

## Development Notes

- Metric namespace is "gnatsd" (not "nats") for backward compatibility with existing dashboards
- **Helm override**: `nats-io/k8s` Helm charts run the exporter with `-prefix nats`, producing `nats_*` metrics. Any namespace/prefix change must be evaluated against **both** worlds — bare-binary dashboards expect `gnatsd_*`, Helm dashboards expect `nats_*`. Both dashboard variants ship in `walkthrough/`
- All metrics are implemented as gauges (set on each scrape)
- The exporter is designed to run alongside each NATS server (1:1 mapping recommended)
- Server response keys are tracked to detect when NATS server metrics change
- HTTP client has configurable timeout for polling NATS endpoints

## Release & distribution

- **Docker**: `natsio/prometheus-nats-exporter:{latest,version}` on Docker Hub, built via `docker-bake.hcl` + `.github/workflows/release.yaml`
- **Binaries**: GoReleaser (`.goreleaser.yml`) produces multi-arch builds — linux (amd64, arm/v6, arm/v7, arm64, 386, mips64le, s390x), macOS, Windows, FreeBSD
- **Packages**: Debian `.deb` via `nfpms` block in GoReleaser config
- **Downstream**: `nats-io/k8s` Helm charts embed this as a sidecar; changes to flags or metric names ripple to chart users

## GitHub CLI Integration

Use `gh` CLI for GitHub operations:
```bash
gh pr view <number>     # View PR details
gh pr diff <number>     # View PR diff
gh issue list           # List issues
```
