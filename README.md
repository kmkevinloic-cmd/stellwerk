# Stellwerk

**A fail-operational runtime for edge-AI robot fleets.**

A *Stellwerk* is a railway signal box: the safety-critical control point that keeps
many moving units running without ever letting them collide. Stellwerk applies the
same idea to fleets of robots that run AI models at the edge: every node supervises
its own perception and inference pipeline, degrades gracefully when deadlines are
missed, and reports a truthful health state to a control plane that keeps a live,
conservative picture of the whole fleet.

The design borrows from automotive functional safety (ISO 26262: debounced fault
detection, latched safe states, hysteresis on recovery) and applies it to
fleet-scale robotics and edge inference, where these patterns are still rare.

> Status: **v0.1, early development.** The safety core, the control plane and the
> fleet simulator work end to end. The inference runtime (ONNX Runtime / TensorRT
> backends) is next on the roadmap.

## Components

| Component | Language | What it does |
|---|---|---|
| [`edge/`](edge/) | C++17 | On-robot safety library: deadline watchdogs, debounced fault monitors, a degradation state machine (`NOMINAL → DEGRADED → MINIMAL_RISK`) and a UDP heartbeat publisher. |
| [`control-plane/`](control-plane/) | Go | `stellwerkd`: ingests heartbeats, tracks liveness per node (`ALIVE / SUSPECT / LOST`), detects sequence gaps and restarts, and serves the fleet picture over HTTP. |
| [`sim/`](sim/) | Python | Fleet simulator: spawns hundreds of virtual robots with configurable fault injection (latency spikes, crashes, packet loss, network partitions). |

```
   robot / edge node                         control plane
 ┌──────────────────────────┐            ┌───────────────────────┐
 │ perception ─► inference  │            │ stellwerkd            │
 │      │            │      │  UDP/JSON  │  ingest ─► registry   │
 │  watchdogs   fault mons  │ ─────────► │            │          │
 │      └──► degradation SM │ heartbeats │  liveness  ▼          │
 │               │          │            │  HTTP API /v1/...     │
 │        heartbeat pub     │            └───────────────────────┘
 └──────────────────────────┘
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md),
[docs/SAFETY_CONCEPT.md](docs/SAFETY_CONCEPT.md) and
[docs/PROTOCOL.md](docs/PROTOCOL.md).

## Quick start

Requirements: Go ≥ 1.22, Python ≥ 3.10, CMake ≥ 3.16 and a C++17 compiler.
No other dependencies for the runtime (tests fetch GoogleTest).

```bash
# 1. start the control plane
cd control-plane && go run ./cmd/stellwerkd --udp :7400 --http :7401

# 2. in another terminal: 200 virtual robots, 2 % packet loss, random faults
cd sim && python -m stellwerk_sim --nodes 200 --loss 0.02 --fault-rate 0.01

# 3. look at the fleet
curl -s localhost:7401/v1/fleet/summary | python -m json.tool
curl -s 'localhost:7401/v1/nodes?liveness=LOST'
```

Build and test everything:

```bash
make test
```

## Design principles

1. **The node is the safety authority for itself.** The control plane never
   assumes a node is safe; it only observes what the node reports and how recently.
2. **Trust receive time, not sender clocks.** Liveness uses the control plane's
   monotonic clock; node timestamps are informational only.
3. **Debounce before reacting, latch before recovering.** A single late frame is
   noise; a pattern is a fault. A safe stop is never left automatically.
4. **Degrade, don't die.** Every fault maps to a defined reduced mode with a
   defined capability, not to a crash.
5. **Runs without a GPU.** Every safety behaviour is testable on a laptop.

## Roadmap

- [x] Edge safety core: watchdogs, fault monitors, degradation state machine
- [x] Control plane: heartbeat ingest, liveness tracking, HTTP API
- [x] Fleet simulator with fault injection
- [ ] Edge inference runtime: ONNX Runtime CPU backend, latency budgets, dynamic batching
- [ ] TensorRT backend for NVIDIA Jetson
- [ ] Command channel: control plane → node (release from safe stop, mode requests)
- [ ] NATS transport alongside UDP
- [ ] Zone-level policies (fleet degradation when a zone loses too many nodes)
- [ ] Prometheus metrics and a live fleet dashboard
- [ ] ROS 2 bridge

## License

Apache License 2.0. See [LICENSE](LICENSE).
