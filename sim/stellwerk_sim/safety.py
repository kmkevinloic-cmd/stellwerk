"""Python mirror of the edge safety core (edge/src).

Kept deliberately small and behaviour-identical to the C++ library so the
simulator produces the same mode sequences a real node would. The C++ unit
tests are the reference; tests/test_safety.py checks the same scenarios.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum

FDC_MAX = 127
FDC_MIN = -128


class Mode(str, Enum):
    NOMINAL = "NOMINAL"
    DEGRADED = "DEGRADED"
    MINIMAL_RISK = "MINIMAL_RISK"


class Severity(Enum):
    DEGRADE = "DEGRADE"
    MINIMAL_RISK = "MINIMAL_RISK"


@dataclass
class FaultMonitor:
    """Fault detection counter with jump-up debouncing."""

    name: str
    severity: Severity
    fail_step: int = 64
    pass_step: int = 32
    fdc: int = 0
    confirmed: bool = False

    def __post_init__(self) -> None:
        if not self.name:
            raise ValueError("fault name must not be empty")
        if not (1 <= self.fail_step <= 255 and 1 <= self.pass_step <= 255):
            raise ValueError("debounce steps must be in [1, 255]")

    def report(self, failed: bool) -> None:
        if failed:
            self.fdc = min(FDC_MAX, max(self.fdc, 0) + self.fail_step)
            if self.fdc == FDC_MAX:
                self.confirmed = True
        else:
            self.fdc = max(FDC_MIN, self.fdc - self.pass_step)
            if self.fdc == FDC_MIN:
                self.confirmed = False


@dataclass
class DegradationManager:
    """Mode transition rules from docs/SAFETY_CONCEPT.md."""

    recovery_cycles: int = 20
    mode: Mode = Mode.NOMINAL
    healthy: int = 0

    def __post_init__(self) -> None:
        self.recovery_cycles = max(1, self.recovery_cycles)

    def evaluate(self, any_critical: bool, any_degrading: bool) -> Mode:
        if any_critical:
            self.mode, self.healthy = Mode.MINIMAL_RISK, 0
        elif self.mode is Mode.MINIMAL_RISK:
            pass  # latched
        elif any_degrading:
            self.mode, self.healthy = Mode.DEGRADED, 0
        elif self.mode is Mode.DEGRADED:
            self.healthy += 1
            if self.healthy >= self.recovery_cycles:
                self.mode, self.healthy = Mode.NOMINAL, 0
        return self.mode

    def release(self, any_critical: bool) -> bool:
        if self.mode is not Mode.MINIMAL_RISK or any_critical:
            return False
        self.mode, self.healthy = Mode.DEGRADED, 0
        return True


@dataclass
class Supervisor:
    """Fault monitors plus the degradation manager, as in edge/src/supervisor.cpp."""

    recovery_cycles: int = 20
    faults: dict[str, FaultMonitor] = field(default_factory=dict)

    def __post_init__(self) -> None:
        self.manager = DegradationManager(self.recovery_cycles)

    def add_fault(self, name: str, severity: Severity) -> None:
        if name in self.faults:
            raise ValueError(f"duplicate fault name: {name}")
        self.faults[name] = FaultMonitor(name, severity)

    def report(self, name: str, failed: bool) -> None:
        self.faults[name].report(failed)

    def cycle(self) -> Mode:
        return self.manager.evaluate(
            self._any(Severity.MINIMAL_RISK), self._any(Severity.DEGRADE)
        )

    def release(self) -> bool:
        return self.manager.release(self._any(Severity.MINIMAL_RISK))

    @property
    def mode(self) -> Mode:
        return self.manager.mode

    def confirmed_faults(self) -> list[str]:
        return [f.name for f in self.faults.values() if f.confirmed]

    def _any(self, severity: Severity) -> bool:
        return any(f.confirmed for f in self.faults.values() if f.severity is severity)
