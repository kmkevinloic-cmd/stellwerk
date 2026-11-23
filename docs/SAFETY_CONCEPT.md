# Safety concept

This document describes the safety behaviour Stellwerk implements. It borrows
vocabulary from ISO 26262 and AUTOSAR, but Stellwerk is **not** a certified
safety component. It is a reference implementation of the patterns, meant to be
read, tested and adapted.

## Node modes

| Mode | Meaning | Typical capability |
|---|---|---|
| `NOMINAL` | All monitored functions healthy. | Full speed, full autonomy. |
| `DEGRADED` | At least one non-critical fault is confirmed. | Reduced speed, conservative planning, no new tasks. |
| `MINIMAL_RISK` | A critical fault is confirmed. | Controlled stop, hold position. |

What each mode means physically is the application's decision. Stellwerk
guarantees *when* a mode is entered and left, not *how* the robot behaves in it.

## Transitions

```
            DEGRADE fault confirmed
  NOMINAL ─────────────────────────► DEGRADED
     ▲   ◄─────────────────────────     │
     │    all faults healed for N        │ MINIMAL_RISK fault confirmed
     │    consecutive cycles             ▼
     │                              MINIMAL_RISK  (latched)
     │                                   │
     └──── (via DEGRADED) ◄──────────────┘ explicit release() and no active
                                             critical fault
```

Rules, evaluated once per supervision cycle:

1. If any `MINIMAL_RISK` fault is confirmed, the mode is `MINIMAL_RISK`. This
   wins over everything else, from any mode.
2. `MINIMAL_RISK` is **latched**: it is not left when the fault heals. Only an
   explicit `release()` leaves it, and the release is refused while any critical
   fault is still confirmed. A released node goes to `DEGRADED`, not `NOMINAL`,
   and must earn its way back.
3. If any `DEGRADE` fault is confirmed, the mode is at least `DEGRADED`.
4. A `DEGRADED` node returns to `NOMINAL` only after **N consecutive cycles**
   (`recovery_cycles`) with no confirmed fault. Any confirmed fault during that
   window resets the count. This hysteresis prevents mode flapping.

## Fault debouncing

Every fault monitor keeps a fault detection counter (FDC) in `[-128, 127]`:

* a *failed* report adds `fail_step`; a *passed* report subtracts `pass_step`;
* on a failed report while the counter is negative, the counter first jumps to
  0 ("jump up"), so a fault is not masked by a long healthy history;
* the fault becomes **confirmed** when the counter reaches `+127`, and
  **healed** when it reaches `-128`.

With `fail_step = 64`, two consecutive failures confirm a fault; with
`pass_step = 32`, eight consecutive passes heal it. Choosing these steps is the
trade-off between reaction time and false alarms, and is made per fault.

## Control-plane view

The control plane never upgrades a node's safety state. It reports what the
node said and, independently, how fresh that information is:

| Liveness | Condition (receive time on the control plane) |
|---|---|
| `ALIVE` | last heartbeat within `suspect_after × period` |
| `SUSPECT` | silent for at least `suspect_after × period` (default 3) |
| `LOST` | silent for at least `lost_after × period` (default 10) |

A `LOST` node's last reported mode is stale and must be treated as unknown.
Consumers that make decisions (traffic management, task allocation) should
combine both dimensions: a node is only *usable* when it is `ALIVE` and
`NOMINAL`.

Sequence numbers and a per-boot identifier let the control plane detect lost
packets, reordering and silent restarts. A restart is visible even when it is
faster than the liveness timeout.
