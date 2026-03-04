# pon-exporter

Standalone Prometheus exporter for GPON optical signal monitoring. Polls OLTs via [nano-southbound](https://github.com/nanoncore/nano-southbound) and exposes per-ONU optical power, OLT health, and PON port metrics.

Built for the RIPE 92 presentation — gives the community a production-ready tool for GPON monitoring without needing the full nanoncore stack.

## Features

- **Multi-vendor**: Supports all nano-southbound vendors (Huawei, Nokia, ZTE, V-SOL, C-Data, FiberHome, Calix, Adtran, DZS, Ericsson, Cisco, Juniper)
- **Cached polling**: Background goroutines poll OLTs on interval; `/metrics` reads cached snapshot (fast, non-blocking scrapes)
- **27 metrics**: OLT status, PON port power, per-ONU Rx/Tx/distance/temperature, traffic counters
- **Hot reload**: `SIGHUP` reloads config without restart
- **Custom labels**: `gpon_target_info` with user-defined labels for PromQL joins
- **Health/readiness**: Kubernetes-compatible `/-/healthy` and `/-/ready` endpoints

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                         pon-exporter                             │
│                                                                  │
│  ┌──────────┐    ┌─────────────────────────────────────────┐     │
│  │  Config   │    │           Poller Manager                │     │
│  │  Loader   │───▶│                                         │     │
│  │ (YAML)    │    │  ┌─────────┐ ┌─────────┐ ┌─────────┐  │     │
│  └──────────┘    │  │ Target  │ │ Target  │ │ Target  │  │     │
│       │          │  │ Gorout. │ │ Gorout. │ │ Gorout. │  │     │
│       │ SIGHUP   │  │ olt-1   │ │ olt-2   │ │ olt-N   │  │     │
│       │ reload   │  └────┬────┘ └────┬────┘ └────┬────┘  │     │
│       ▼          │       │           │           │        │     │
│  ┌──────────┐    │       ▼           ▼           ▼        │     │
│  │  main.go │    │  ┌─────────────────────────────────┐   │     │
│  │  signals, │    │  │     nano-southbound drivers     │   │     │
│  │  HTTP     │    │  │  Connect → Poll → Disconnect    │   │     │
│  └──────────┘    │  │  (per-poll connection lifecycle) │   │     │
│       │          │  └──────────────┬──────────────────┘   │     │
│       │          │                 │                       │     │
│       │          │                 ▼                       │     │
│       │          │  ┌─────────────────────────────────┐   │     │
│       │          │  │       Snapshot Store             │   │     │
│       │          │  │  (thread-safe, per-target cache) │   │     │
│       │          │  └──────────────┬──────────────────┘   │     │
│       │          └─────────────────│───────────────────────┘     │
│       │                            │                             │
│       │          ┌─────────────────▼──────────────────┐          │
│       │          │        GPON Collector               │          │
│       │          │  (prometheus.Collector interface)    │          │
│       │          │  MustNewConstMetric per Collect()    │          │
│       │          └─────────────────┬──────────────────┘          │
│       │                            │                             │
│       ▼                            ▼                             │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    HTTP Server (:9876)                     │    │
│  │  /metrics   /-/healthy   /-/ready   /                     │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
         ▲                                          │
         │           Prometheus scrape              │
         └──────────────────────────────────────────┘
```

### How It Works

1. **Config loading**: On startup (and on `SIGHUP`), the exporter reads `pon-exporter.yml` which defines OLT targets with vendor, protocol, credentials, and custom labels.

2. **Background polling**: The Poller Manager spawns one goroutine per target with staggered start times to avoid thundering herd. Each goroutine runs a poll loop at the configured `poll_interval` (default 5m).

3. **Per-poll connection lifecycle**: Each poll cycle creates a fresh driver via `nano-southbound.NewDriver()`, connects to the OLT, collects data, and disconnects. This avoids stale SSH/SNMP sessions. Three independent calls are made per poll:
   - `GetOLTStatus()` — CPU, memory, temperature, uptime, PON port status
   - `GetONUList()` — all provisioned ONUs with Rx/Tx power, distance, temperature, traffic counters
   - `GetAlarms()` — active OLT alarms by severity

   Each call fails independently — a `GetAlarms` failure doesn't block ONU data.

4. **DriverV2 type assertion**: `NewDriver()` returns a base `Driver` interface. The exporter type-asserts to `DriverV2` (which adds `GetONUList`, `GetOLTStatus`, `GetAlarms`). If the driver doesn't implement `DriverV2`, the target reports `gpon_driver_v2_supported=0` and skips detailed metrics.

5. **Snapshot store**: Poll results are stored in a thread-safe `SnapshotStore` (one snapshot per target). The store is the bridge between the polling goroutines and the Prometheus collector.

6. **Custom Collector**: The GPON Collector implements `prometheus.Collector` and reads all snapshots on each `Collect()` call. It uses `MustNewConstMetric` to build metrics fresh each time — this means when ONUs disappear between polls, their stale metrics are automatically cleaned up (no label staleness).

7. **`/metrics` endpoint**: Prometheus scrapes `/metrics` which triggers `Collect()`. Since data is pre-cached in the snapshot store, scrapes are fast and non-blocking regardless of how long OLT polls take.

## Getting Started

### Prerequisites

- **Go 1.24+** (for building from source)
- Network access to your OLT management interfaces (SSH port 22, SNMP port 161, etc.)
- OLT credentials with read access

### Step 1: Install

**Option A — Build from source:**
```bash
git clone https://github.com/nanoncore/pon-exporter.git
cd pon-exporter
go build -o pon-exporter ./cmd/pon-exporter
```

**Option B — Go install:**
```bash
go install github.com/nanoncore/pon-exporter/cmd/pon-exporter@latest
```

**Option C — Docker:**
```bash
docker pull ghcr.io/nanoncore/pon-exporter:latest
```

**Option D — Download binary:**

Grab the latest release from [GitHub Releases](https://github.com/nanoncore/pon-exporter/releases).

### Step 2: Configure

Create a `pon-exporter.yml` file:

```yaml
poll_interval: 5m

targets:
  - name: olt-rack1
    vendor: vsol        # see Supported Vendors table below
    protocol: cli       # cli, snmp, netconf, or gnmi
    address: 192.168.1.1
    port: 22
    username: admin
    password: admin
    timeout: 30s
    labels:             # optional: custom Prometheus labels
      site: main-pop
      rack: "1"

  - name: olt-rack2
    vendor: huawei
    protocol: snmp
    address: 192.168.1.2
    port: 161
    snmp_community: public
    snmp_version: "2c"
    timeout: 60s
    labels:
      site: main-pop
      rack: "2"
```

**Key config options:**
- `poll_interval`: How often to poll each OLT (minimum 10s, recommended 5m for production)
- `vendor`: Must match a supported vendor (see table below)
- `protocol`: The management protocol to use — `cli` (SSH), `snmp`, `netconf`, or `gnmi`
- `labels`: Arbitrary key-value pairs exposed as `gpon_target_info` for PromQL joins

### Step 3: Run

```bash
./pon-exporter --config.file=pon-exporter.yml
```

Verify it's working:
```bash
# Check health
curl http://localhost:9876/-/healthy

# Check readiness (returns 503 until first poll completes, then 200)
curl http://localhost:9876/-/ready

# View metrics
curl http://localhost:9876/metrics
```

### Step 4: Configure Prometheus

Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: pon-exporter
    scrape_interval: 30s
    static_configs:
      - targets: ["localhost:9876"]
```

### Step 5: Import Grafana Dashboard

Import the included dashboard from `dashboards/gpon-overview.json` (Grafana UI > Dashboards > Import > Upload JSON).

### Step 6: (Optional) Hot Reload

Update `pon-exporter.yml` and send SIGHUP to reload without restart:

```bash
kill -HUP $(pidof pon-exporter)
```

New targets are added, removed targets are cleaned up, and existing targets continue with updated config.

## Docker Compose (Full Stack)

Run pon-exporter + Prometheus + Grafana with one command:

```bash
docker compose up -d
```

Access:
- **Exporter**: http://localhost:9876/metrics
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)

## Supported Vendors & Protocols

| Vendor | CLI | SNMP | NETCONF | gNMI |
|--------|-----|------|---------|------|
| Huawei | ✓ | ✓ | ✓ | |
| Nokia | ✓ | | ✓ | ✓ |
| ZTE | ✓ | ✓ | | |
| V-SOL | ✓ | ✓ | | |
| C-Data | ✓ | ✓ | | |
| FiberHome | ✓ | ✓ | | |
| Cisco | ✓ | | ✓ | ✓ |
| Juniper | ✓ | | ✓ | ✓ |
| Calix | ✓ | | ✓ | |
| Adtran | | | ✓ | |
| DZS | ✓ | ✓ | | |
| Ericsson | ✓ | | ✓ | |

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config.file` | `pon-exporter.yml` | Configuration file path |
| `--web.listen-address` | `:9876` | Listen address |
| `--web.config.file` | | TLS/basic-auth config ([exporter-toolkit](https://github.com/prometheus/exporter-toolkit)) |
| `--log.level` | `info` | Log level (debug, info, warn, error) |
| `--log.format` | `logfmt` | Log format (logfmt, json) |
| `--version` | | Show version |

## HTTP Endpoints

| Path | Description |
|------|-------------|
| `/metrics` | Prometheus metrics |
| `/-/healthy` | Liveness probe (always 200) |
| `/-/ready` | Readiness probe (200 after first poll, 503 before) |
| `/` | Landing page |

## Metrics Reference

### Target-level

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gpon_up` | Gauge | target | OLT reachable (1/0) |
| `gpon_driver_v2_supported` | Gauge | target | Driver supports DriverV2 (1/0) |
| `gpon_scrape_duration_seconds` | Gauge | target | Last poll duration |
| `gpon_scrape_errors_total` | Counter | target | Cumulative poll errors |
| `gpon_target_info` | Gauge | target + custom | Static info, always 1 |

### OLT-level

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gpon_olt_info` | Gauge | target, vendor, model, firmware, serial_number | Static OLT info, always 1 |
| `gpon_olt_cpu_percent` | Gauge | target | CPU utilization |
| `gpon_olt_memory_percent` | Gauge | target | Memory utilization |
| `gpon_olt_temperature_celsius` | Gauge | target | System temperature |
| `gpon_olt_active_onus` | Gauge | target | Online ONUs |
| `gpon_olt_total_onus` | Gauge | target | All provisioned ONUs |
| `gpon_olt_uptime_seconds` | Gauge | target | OLT uptime |
| `gpon_olt_active_alarms` | Gauge | target | Total active alarms |
| `gpon_olt_alarms_by_severity` | Gauge | target, severity | Alarms by severity |

### PON Port-level

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gpon_pon_port_onu_count` | Gauge | target, pon_port | ONUs on port |
| `gpon_pon_port_rx_power_dbm` | Gauge | target, pon_port | Port Rx power |
| `gpon_pon_port_tx_power_dbm` | Gauge | target, pon_port | Port Tx power |

### ONU-level

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `gpon_onu_info` | Gauge | target, pon_port, onu_id, serial, oper_state, admin_state, vendor, model | Static ONU info, always 1 |
| `gpon_onu_online` | Gauge | target, pon_port, onu_id, serial | ONU online (1/0) |
| `gpon_onu_rx_power_dbm` | Gauge | target, pon_port, onu_id, serial | ONU Rx power |
| `gpon_onu_tx_power_dbm` | Gauge | target, pon_port, onu_id, serial | ONU Tx power |
| `gpon_onu_olt_rx_power_dbm` | Gauge | target, pon_port, onu_id, serial | OLT Rx from ONU |
| `gpon_onu_distance_meters` | Gauge | target, pon_port, onu_id, serial | Fiber distance |
| `gpon_onu_temperature_celsius` | Gauge | target, pon_port, onu_id, serial | ONU temperature |
| `gpon_onu_signal_within_spec` | Gauge | target, pon_port, onu_id, serial | Signal within GPON spec (1/0) |
| `gpon_onu_bytes_up_total` | Counter | target, pon_port, onu_id, serial | Upstream bytes |
| `gpon_onu_bytes_down_total` | Counter | target, pon_port, onu_id, serial | Downstream bytes |

## PromQL Examples

```promql
# ONUs with signal out of spec
gpon_onu_signal_within_spec == 0

# Average ONU Rx power per OLT
avg by (target) (gpon_onu_rx_power_dbm)

# OLTs with high CPU
gpon_olt_cpu_percent > 80

# ONU count per site (using target_info join)
sum by (site) (
  gpon_olt_active_onus
  * on(target) group_left(site)
  gpon_target_info
)

# ONUs with degrading signal (Rx power below -25 dBm)
gpon_onu_rx_power_dbm < -25

# Traffic throughput per ONU (rate over 5m)
rate(gpon_onu_bytes_up_total[5m]) * 8  # bits/sec
```

## Project Structure

```
pon-exporter/
├── cmd/pon-exporter/main.go      # Entry point, CLI, HTTP, signal handling
├── internal/
│   ├── config/                    # YAML config types + loader + validation
│   ├── collector/                 # Prometheus Collector (Describe + Collect)
│   ├── poller/                    # Background polling manager + snapshot store
│   ├── target/                    # Single-target poll logic (driver lifecycle)
│   └── version/                   # Build version ldflags
├── dashboards/                    # Grafana dashboard JSON
├── Dockerfile                     # Multi-stage build
├── docker-compose.yml             # Full observability stack
├── .goreleaser.yml                # Release automation
└── pon-exporter.yml               # Example config
```

## License

Apache 2.0. See [LICENSE](LICENSE).
