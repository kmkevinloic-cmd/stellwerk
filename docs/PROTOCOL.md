# Heartbeat protocol (v1)

Each node sends one heartbeat per supervision period as a single UDP datagram
containing one UTF-8 JSON object. Datagrams larger than **1400 bytes** are
rejected so a heartbeat never fragments on a standard Ethernet MTU.

## Message

```json
{
  "v": 1,
  "node_id": "amr-0042",
  "boot_id": "9f1c2a7e5b3d4c11",
  "seq": 18231,
  "period_ms": 100,
  "uptime_ms": 1823100,
  "mode": "DEGRADED",
  "faults": ["infer_deadline"],
  "metrics": { "infer_p99_ms": 41.7, "battery": 0.63 }
}
```

| Field | Type | Rules |
|---|---|---|
| `v` | integer | Must be `1`. |
| `node_id` | string | 1–64 chars, `[A-Za-z0-9._-]`. Stable across restarts. |
| `boot_id` | string | 1–32 chars. New random value on every process start. |
| `seq` | integer | Starts at 0 on every boot, +1 per heartbeat. |
| `period_ms` | integer | 10–60000. The node's declared heartbeat period. |
| `uptime_ms` | integer | ≥ 0. Informational. |
| `mode` | string | `NOMINAL`, `DEGRADED` or `MINIMAL_RISK`. |
| `faults` | string[] | Confirmed faults, at most 32 entries. May be empty or omitted. |
| `metrics` | object | String → number, at most 32 entries. May be omitted. |

Unknown fields are ignored, so v1 can be extended compatibly.

## Receiver behaviour

For each node the control plane keeps `(boot_id, last_seq)`:

* **new `boot_id`**: the node restarted; counters reset, `restarts` +1;
* **`seq > last_seq + 1`**: `seq − last_seq − 1` heartbeats were lost;
* **`seq ≤ last_seq`** with the same `boot_id`: duplicate or reordered, counted
  and **ignored** so stale state never overwrites newer state.

Liveness is computed from the control plane's monotonic receive time and the
node's declared `period_ms` (see [SAFETY_CONCEPT.md](SAFETY_CONCEPT.md)).

## Security

v1 has no authentication: run it on a trusted network segment. Signed
heartbeats are on the roadmap.
