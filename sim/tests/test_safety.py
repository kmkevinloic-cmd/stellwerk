"""Same scenarios as edge/tests, to keep the Python mirror faithful."""

import unittest

from stellwerk_sim.safety import (
    FDC_MAX,
    FDC_MIN,
    DegradationManager,
    FaultMonitor,
    Mode,
    Severity,
    Supervisor,
)


class FaultMonitorTest(unittest.TestCase):
    def test_two_failures_confirm(self):
        f = FaultMonitor("x", Severity.DEGRADE)
        f.report(True)
        self.assertFalse(f.confirmed)
        self.assertEqual(f.fdc, 64)
        f.report(True)
        self.assertTrue(f.confirmed)
        self.assertEqual(f.fdc, FDC_MAX)

    def test_eight_passes_heal(self):
        f = FaultMonitor("x", Severity.DEGRADE)
        f.report(True)
        f.report(True)
        for _ in range(7):
            f.report(False)
            self.assertTrue(f.confirmed)
        f.report(False)
        self.assertFalse(f.confirmed)
        self.assertEqual(f.fdc, FDC_MIN)

    def test_jump_up(self):
        f = FaultMonitor("x", Severity.DEGRADE)
        for _ in range(100):
            f.report(False)
        f.report(True)
        self.assertEqual(f.fdc, 64)

    def test_alternating_noise_never_confirms(self):
        f = FaultMonitor("x", Severity.DEGRADE, fail_step=32, pass_step=32)
        for i in range(1000):
            f.report(i % 2 == 0)
        self.assertFalse(f.confirmed)

    def test_invalid_config(self):
        with self.assertRaises(ValueError):
            FaultMonitor("", Severity.DEGRADE)
        with self.assertRaises(ValueError):
            FaultMonitor("x", Severity.DEGRADE, fail_step=0)


class DegradationTest(unittest.TestCase):
    def test_recovery_hysteresis(self):
        m = DegradationManager(5)
        m.evaluate(False, True)
        for _ in range(4):
            self.assertIs(m.evaluate(False, False), Mode.DEGRADED)
        m.evaluate(False, True)
        for _ in range(4):
            self.assertIs(m.evaluate(False, False), Mode.DEGRADED)
        self.assertIs(m.evaluate(False, False), Mode.NOMINAL)

    def test_minimal_risk_latched_and_release(self):
        m = DegradationManager(3)
        m.evaluate(True, False)
        for _ in range(50):
            self.assertIs(m.evaluate(False, False), Mode.MINIMAL_RISK)
        self.assertFalse(m.release(True))
        self.assertTrue(m.release(False))
        self.assertIs(m.mode, Mode.DEGRADED)

    def test_release_outside_minimal_risk_refused(self):
        m = DegradationManager(3)
        self.assertFalse(m.release(False))


class SupervisorTest(unittest.TestCase):
    def test_latency_degrade_and_recover_in_12_cycles(self):
        s = Supervisor(5)
        s.add_fault("infer_deadline", Severity.DEGRADE)
        for _ in range(2):
            s.report("infer_deadline", True)
            s.cycle()
        self.assertIs(s.mode, Mode.DEGRADED)
        cycles = 0
        while s.mode is not Mode.NOMINAL and cycles < 100:
            s.report("infer_deadline", False)
            s.cycle()
            cycles += 1
        self.assertEqual(cycles, 12)  # identical to edge/tests/supervisor_test.cpp

    def test_critical_overrides_degrade(self):
        s = Supervisor(5)
        s.add_fault("a", Severity.DEGRADE)
        s.add_fault("b", Severity.MINIMAL_RISK)
        for _ in range(2):
            s.report("a", True)
            s.report("b", True)
        self.assertIs(s.cycle(), Mode.MINIMAL_RISK)
        self.assertEqual(sorted(s.confirmed_faults()), ["a", "b"])

    def test_duplicate_fault(self):
        s = Supervisor()
        s.add_fault("x", Severity.DEGRADE)
        with self.assertRaises(ValueError):
            s.add_fault("x", Severity.DEGRADE)


if __name__ == "__main__":
    unittest.main()
