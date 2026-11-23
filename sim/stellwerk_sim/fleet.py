"""Runs many virtual nodes against a control plane over UDP."""

from __future__ import annotations

import random
import socket
import sys
import time
from collections import Counter
from dataclasses import dataclass, field
from typing import Callable, Optional, TextIO

from .node import FaultKind, NodeConfig, VirtualNode

# Relative likelihood and duration range (seconds) of each random fault.
FAULT_PROFILE: dict[FaultKind, tuple[float, tuple[float, float]]] = {
    FaultKind.LATENCY_SPIKE: (0.5, (1.0, 5.0)),
    FaultKind.CAMERA_STALL: (0.2, (1.0, 3.0)),
    FaultKind.CRASH: (0.1, (2.0, 8.0)),
    FaultKind.PARTITION: (0.2, (1.0, 5.0)),
}


@dataclass
class FleetConfig:
    nodes: int = 50
    prefix: str = "sim"
    fault_rate: float = 0.0  # probability per node per second of a new fault
    seed: Optional[int] = None
    node: NodeConfig = field(default_factory=NodeConfig)


class Fleet:
    def __init__(self, cfg: FleetConfig) -> None:
        self.cfg = cfg
        self.rng = random.Random(cfg.seed)
        width = max(4, len(str(cfg.nodes)))
        self.nodes = [
            VirtualNode(
                f"{cfg.prefix}-{i:0{width}d}",
                cfg.node,
                random.Random(self.rng.random()),
            )
            for i in range(cfg.nodes)
        ]
        self.injected: Counter[FaultKind] = Counter()

    def maybe_inject(self, t: float) -> None:
        p = self.cfg.fault_rate * self.cfg.node.period_s
        if p <= 0:
            return
        kinds = list(FAULT_PROFILE)
        weights = [FAULT_PROFILE[k][0] for k in kinds]
        for node in self.nodes:
            if self.rng.random() < p:
                kind = self.rng.choices(kinds, weights)[0]
                lo, hi = FAULT_PROFILE[kind][1]
                node.inject(kind, t, self.rng.uniform(lo, hi))
                self.injected[kind] += 1

    def step(self, t: float, send: Callable[[bytes], None]) -> int:
        self.maybe_inject(t)
        sent = 0
        for node in self.nodes:
            data = node.step(t)
            if data is not None:
                send(data)
                sent += 1
        return sent

    def mode_counts(self) -> Counter[str]:
        return Counter(n.mode.value for n in self.nodes)


def run(
    cfg: FleetConfig,
    target: tuple[str, int],
    duration_s: float = 0.0,
    report_every_s: float = 5.0,
    out: TextIO = sys.stdout,
) -> int:
    """Run the fleet in real time. Returns the number of datagrams sent."""
    fleet = Fleet(cfg)
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)

    def send(data: bytes) -> None:
        try:
            sock.sendto(data, target)
        except OSError:
            pass  # a lost heartbeat is replaced by the next one

    period = cfg.node.period_s
    start = time.monotonic()
    next_tick = start
    next_report = start + report_every_s
    total = 0
    print(
        f"simulating {cfg.nodes} nodes -> {target[0]}:{target[1]} "
        f"(period {period * 1000:.0f} ms, loss {cfg.node.loss:.1%}, "
        f"fault rate {cfg.fault_rate}/node/s)",
        file=out,
        flush=True,
    )
    try:
        while True:
            now = time.monotonic()
            t = now - start
            if duration_s and t >= duration_s:
                break
            total += fleet.step(t, send)
            if now >= next_report:
                modes = fleet.mode_counts()
                print(
                    f"[{t:7.1f}s] sent={total} "
                    + " ".join(f"{m}={modes.get(m, 0)}" for m in ("NOMINAL", "DEGRADED", "MINIMAL_RISK"))
                    + " injected="
                    + ",".join(f"{k.value}:{v}" for k, v in sorted(fleet.injected.items())),
                    file=out,
                    flush=True,
                )
                next_report += report_every_s
            next_tick += period
            delay = next_tick - time.monotonic()
            if delay > 0:
                time.sleep(delay)
            else:
                next_tick = time.monotonic()  # overloaded: do not try to catch up
    except KeyboardInterrupt:
        pass
    finally:
        sock.close()
    return total
