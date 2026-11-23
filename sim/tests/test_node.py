import json
import random
import unittest

from stellwerk_sim.fleet import Fleet, FleetConfig
from stellwerk_sim.node import FaultKind, NodeConfig, VirtualNode
from stellwerk_sim.safety import Mode

PERIOD = 0.1


def make(release_after=None, loss=0.0, seed=1):
    cfg = NodeConfig(period_s=PERIOD, recovery_cycles=5, loss=loss, release_after_s=release_after)
    return VirtualNode("n-1", cfg, random.Random(seed))


def run(node, start, end):
    """Step node over [start, end) and return the decoded heartbeats (None if not sent)."""
    out = []
    steps = round((end - start) / PERIOD)
    for i in range(steps):
        data = node.step(start + i * PERIOD)
        out.append(None if data is None else json.loads(data))
    return out


class VirtualNodeTest(unittest.TestCase):
    def test_heartbeat_matches_protocol(self):
        hb = run(make(), 0.0, 0.1)[0]
        self.assertEqual(hb["v"], 1)
        self.assertEqual(hb["node_id"], "n-1")
        self.assertEqual(len(hb["boot_id"]), 16)
        self.assertEqual(hb["seq"], 0)
        self.assertEqual(hb["period_ms"], 100)
        self.assertEqual(hb["mode"], "NOMINAL")
        self.assertEqual(hb["faults"], [])
        self.assertIn("infer_ms", hb["metrics"])

    def test_latency_spike_degrades_then_recovers(self):
        n = make()
        n.inject(FaultKind.LATENCY_SPIKE, 1.0, 1.0)
        hbs = run(n, 0.0, 6.0)
        modes = [h["mode"] for h in hbs]
        self.assertEqual(modes[9], "NOMINAL")
        self.assertIn("DEGRADED", modes[10:20])
        self.assertNotIn("MINIMAL_RISK", modes)
        self.assertEqual(modes[-1], "NOMINAL")

    def test_camera_stall_is_latched_without_operator(self):
        n = make(release_after=None)
        n.inject(FaultKind.CAMERA_STALL, 1.0, 1.0)
        hbs = run(n, 0.0, 10.0)
        self.assertEqual(hbs[-1]["mode"], "MINIMAL_RISK")
        self.assertEqual(hbs[-1]["faults"], [])  # healed, but still latched

    def test_operator_releases_after_delay(self):
        n = make(release_after=2.0)
        n.inject(FaultKind.CAMERA_STALL, 1.0, 1.0)
        run(n, 0.0, 10.0)
        self.assertIs(n.mode, Mode.NOMINAL)

    def test_crash_restarts_with_new_boot_id(self):
        n = make()
        n.inject(FaultKind.CRASH, 1.0, 2.0)
        hbs = run(n, 0.0, 4.0)
        before, during, after = hbs[9], hbs[10:30], hbs[30]
        self.assertTrue(all(h is None for h in during))
        self.assertNotEqual(before["boot_id"], after["boot_id"])
        self.assertEqual(after["seq"], 0)
        self.assertEqual(after["uptime_ms"], 0)

    def test_partition_keeps_sequence_running(self):
        n = make()
        n.inject(FaultKind.PARTITION, 1.0, 1.0)
        hbs = run(n, 0.0, 3.0)
        self.assertTrue(all(h is None for h in hbs[10:20]))
        self.assertEqual(hbs[20]["seq"], 20)
        self.assertEqual(hbs[20]["boot_id"], hbs[0]["boot_id"])

    def test_packet_loss_rate(self):
        n = make(loss=0.2, seed=42)
        hbs = run(n, 0.0, 200.0)
        lost = sum(h is None for h in hbs) / len(hbs)
        self.assertAlmostEqual(lost, 0.2, delta=0.03)


class FleetTest(unittest.TestCase):
    def test_ids_and_determinism(self):
        cfg = FleetConfig(nodes=12, prefix="amr", fault_rate=0.5, seed=7)
        a, b = Fleet(cfg), Fleet(cfg)
        self.assertEqual(a.nodes[0].node_id, "amr-0000")
        self.assertEqual(a.nodes[-1].node_id, "amr-0011")
        for i in range(50):
            a.step(i * PERIOD, lambda _: None)
            b.step(i * PERIOD, lambda _: None)
        self.assertEqual(a.injected, b.injected)
        self.assertEqual(a.mode_counts(), b.mode_counts())
        self.assertGreater(sum(a.injected.values()), 0)

    def test_no_faults_means_all_nominal(self):
        fleet = Fleet(FleetConfig(nodes=20, seed=1))
        sent = []
        for i in range(30):
            fleet.step(i * PERIOD, sent.append)
        self.assertEqual(len(sent), 600)
        self.assertEqual(fleet.mode_counts()["NOMINAL"], 20)


if __name__ == "__main__":
    unittest.main()
