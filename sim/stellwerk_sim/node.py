"""A virtual robot that behaves like an edge node running the Stellwerk core."""

from __future__ import annotations

import json
import random
import secrets
from dataclasses import dataclass, field
from enum import Enum
from typing import Optional

from .safety import Mode, Severity, Supervisor

LATENCY_BUDGET_MS = 50.0
CAMERA_DEADLINE_S = 0.3
MAX_DATAGRAM_BYTES = 1400


class FaultKind(str, Enum):
    LATENCY_SPIKE = "latency_spike"  # inference over budget -> DEGRADED
    CAMERA_STALL = "camera_stall"  # watchdog expiry -> MINIMAL_RISK
    CRASH = "crash"  # process dies, restarts later with a new boot_id
    PARTITION = "partition"  # keeps running, heartbeats do not arrive


@dataclass
class InjectedFault:
    kind: FaultKind
    start: float
    end: float

    def active(self, t: float) -> bool:
        return self.start <= t < self.end


@dataclass
class NodeConfig:
    period_s: float = 0.1
    recovery_cycles: int = 20
    loss: float = 0.0  # probability a heartbeat is dropped in transit
    release_after_s: Optional[float] = 5.0  # simulated operator; None = never


def new_supervisor(recovery_cycles: int) -> Supervisor:
    sup = Supervisor(recovery_cycles)
    sup.add_fault("camera_stall", Severity.MINIMAL_RISK)
    sup.add_fault("infer_deadline", Severity.DEGRADE)
    return sup


@dataclass
class VirtualNode:
    node_id: str
    cfg: NodeConfig
    rng: random.Random
    boot_id: str = field(default_factory=lambda: secrets.token_hex(8))
    seq: int = 0
    boot_time: float = 0.0
    last_frame: float = 0.0
    mrc_since: Optional[float] = None
    faults: list[InjectedFault] = field(default_factory=list)
    _was_down: bool = False

    def __post_init__(self) -> None:
        self.sup = new_supervisor(self.cfg.recovery_cycles)

    # -- fault injection -------------------------------------------------

    def inject(self, kind: FaultKind, start: float, duration: float) -> None:
        self.faults.append(InjectedFault(kind, start, start + duration))

    def _active(self, kind: FaultKind, t: float) -> bool:
        return any(f.kind is kind and f.active(t) for f in self.faults)

    # -- one supervision period ------------------------------------------

    def step(self, t: float) -> Optional[bytes]:
        """Advance to time t. Returns the datagram to send, or None."""
        self.faults = [f for f in self.faults if f.end > t]

        if self._active(FaultKind.CRASH, t):
            self._was_down = True
            return None
        if self._was_down:
            self._restart(t)

        if not self._active(FaultKind.CAMERA_STALL, t):
            self.last_frame = t
        self.sup.report("camera_stall", t - self.last_frame > CAMERA_DEADLINE_S)

        latency = max(1.0, self.rng.gauss(22.0, 4.0))
        if self._active(FaultKind.LATENCY_SPIKE, t):
            latency += 60.0
        self.sup.report("infer_deadline", latency > LATENCY_BUDGET_MS)

        mode = self.sup.cycle()
        mode = self._operator(mode, t)

        hb = self._heartbeat(t, mode, latency)
        self.seq += 1

        if self._active(FaultKind.PARTITION, t) or self.rng.random() < self.cfg.loss:
            return None
        return hb

    def _operator(self, mode: Mode, t: float) -> Mode:
        """Simulated operator: requests release some time after a safe stop."""
        if mode is not Mode.MINIMAL_RISK:
            self.mrc_since = None
            return mode
        if self.mrc_since is None:
            self.mrc_since = t
        wait = self.cfg.release_after_s
        if wait is not None and t - self.mrc_since >= wait and self.sup.release():
            self.mrc_since = None
        return self.sup.mode

    def _restart(self, t: float) -> None:
        self._was_down = False
        self.boot_id = secrets.token_hex(8)
        self.seq = 0
        self.boot_time = t
        self.last_frame = t
        self.mrc_since = None
        self.sup = new_supervisor(self.cfg.recovery_cycles)

    def _heartbeat(self, t: float, mode: Mode, latency: float) -> bytes:
        msg = {
            "v": 1,
            "node_id": self.node_id,
            "boot_id": self.boot_id,
            "seq": self.seq,
            "period_ms": round(self.cfg.period_s * 1000),
            "uptime_ms": round((t - self.boot_time) * 1000),
            "mode": mode.value,
            "faults": self.sup.confirmed_faults(),
            "metrics": {"infer_ms": round(latency, 2)},
        }
        data = json.dumps(msg, separators=(",", ":")).encode()
        if len(data) > MAX_DATAGRAM_BYTES:  # cannot happen with sane ids
            raise ValueError("heartbeat too large")
        return data

    @property
    def mode(self) -> Mode:
        return self.sup.mode
