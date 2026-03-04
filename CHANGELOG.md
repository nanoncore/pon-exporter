# Changelog

## [Unreleased]

### Added
- Initial release of pon-exporter
- Background polling with per-target goroutines and staggered start
- 27 GPON metrics (OLT status, PON port, ONU optical/traffic)
- Custom `gpon_target_info` with user-defined labels
- SIGHUP config reload without restart
- Health (`/-/healthy`) and readiness (`/-/ready`) endpoints
- Docker and docker-compose setup with Prometheus and Grafana
- Grafana dashboard for GPON overview
- Multi-platform builds (linux/darwin, amd64/arm64)
- Apache 2.0 license
