# Changelog

All notable changes to Stellwerk are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/). Release dates are on the
corresponding git tags.

## [Unreleased]

## [0.1.0]

First public release: the safety core, the control plane and the fleet
simulator work end to end.

### Added

- **Docs**: architecture, safety concept and heartbeat protocol v1.
- **Edge (C++17)**: `FaultMonitor` with fault detection counter and jump-up
  debouncing; deadline watchdogs; `DegradationManager` with latched
  `MINIMAL_RISK` and recovery hysteresis; `Supervisor` tying them together;
  `HeartbeatPublisher` over UDP; simulated `edge_node` example.
- **Control plane (Go)**: `stellwerkd` with UDP heartbeat ingest, per-node
  liveness (`ALIVE / SUSPECT / LOST`), sequence-gap, stale-packet and restart
  detection, and a read-only HTTP API (`/v1/fleet/summary`, `/v1/nodes`).
- **Simulator (Python)**: virtual fleet with latency spikes, camera stalls,
  crashes, network partitions and packet loss; standard library only.
- **Build**: Makefile, GitHub Actions CI for all three components, and
  single-source versioning (`VERSION`, `scripts/version.sh`).

[Unreleased]: ../../compare/v0.1.0...HEAD
[0.1.0]: ../../releases/tag/v0.1.0
