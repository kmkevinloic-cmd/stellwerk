# Architecture

Stellwerk has two halves that deliberately share as little as possible: the
**edge** (one process per robot) and the **control plane** (one service per fleet).
They are coupled only by a small, versioned heartbeat protocol
([PROTOCOL.md](PROTOCOL.md)).

## Edge node (`edge/`, C++17)

The edge library is linked into the robot's main process. It has no dynamic
allocation on the supervision path after start-up and no dependency beyond the
C++ standard library and POSIX sockets.

```
            kick()                    report(pass|fail)
 pipeline ─────────► Watchdog ─────────────┐
 stages                                    ▼
                                      FaultMonitor  (debounced, per fault)
 latency  ──── report(pass|fail) ────►     │ confirmed / healed
 checks                                    ▼
                                   DegradationManager  ──► mode
                                           │
                                   HeartbeatPublisher  ──► UDP
```

* **Watchdog**: deadline monitoring for one pipeline stage (camera frame,
  inference result, control loop). The stage calls `kick()`; the supervisor cycle
  calls `check()`, which reports *passed* or *failed* to a fault monitor.
* **FaultMonitor**: turns noisy pass/fail events into a confirmed fault using a
  fault detection counter, in the style of the AUTOSAR diagnostic event manager.
  Each monitor carries a *severity*: `DEGRADE` or `MINIMAL_RISK`.
* **DegradationManager**: owns the node mode. It is evaluated once per
  supervision cycle and is the only component that may change the mode.
* **HeartbeatPublisher**: serialises the mode, active faults and a few metrics
  and sends them to the control plane every supervision period.

The supervision cycle is driven by the application (for example from its main
loop or a dedicated thread at 10–100 Hz). The library takes an injectable clock
so every timing behaviour is unit-testable without sleeping.

## Control plane (`control-plane/`, Go)

`stellwerkd` is a single binary with three parts:

* **ingest**: a UDP listener that decodes and validates heartbeats and hands
  them to the registry. Malformed packets are counted and dropped.
* **fleet registry**: one record per node: last reported mode and faults,
  sequence tracking, restart detection and a liveness state derived purely from
  the control plane's own monotonic clock.
* **api**: a read-only HTTP/JSON view of the registry.

A background ticker re-evaluates liveness for every node, so a node that goes
silent moves `ALIVE → SUSPECT → LOST` even though no packet arrives to trigger it.

## Simulator (`sim/`, Python)

The simulator speaks the same protocol as a real node, so the control plane
cannot tell the difference. Each virtual robot runs a simplified copy of the
edge degradation logic and can be subjected to injected faults: inference
latency spikes, crashes and restarts, packet loss and network partitions. It
uses only the Python standard library.

## Why UDP first

Heartbeats are small, periodic and idempotent: a lost heartbeat is better
replaced by the next one than retransmitted late. UDP keeps the edge side free of
connection state and broker dependencies. A NATS transport is on the roadmap for
fleets that already run a message bus; the registry is transport-agnostic.

## Repository layout

```
docs/            architecture, safety concept, protocol
edge/            C++ library, example node, GoogleTest tests
control-plane/   Go module: cmd/stellwerkd, internal/{protocol,fleet,ingest,api}
sim/             Python package stellwerk_sim and tests
```
