"""Command line entry point: python -m stellwerk_sim --help"""

from __future__ import annotations

import argparse

from .fleet import FleetConfig, run
from .node import NodeConfig


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(prog="stellwerk_sim", description="Stellwerk fleet simulator")
    p.add_argument("--target", default="127.0.0.1:7400", help="control plane UDP address")
    p.add_argument("--nodes", type=int, default=50)
    p.add_argument("--prefix", default="sim", help="node id prefix")
    p.add_argument("--period-ms", type=int, default=100)
    p.add_argument("--loss", type=float, default=0.0, help="heartbeat loss probability")
    p.add_argument("--fault-rate", type=float, default=0.0, help="new faults per node per second")
    p.add_argument("--release-after", type=float, default=5.0,
                   help="seconds before the simulated operator releases a safe stop (<0: never)")
    p.add_argument("--recovery-cycles", type=int, default=20)
    p.add_argument("--duration", type=float, default=0.0, help="seconds to run (0: until Ctrl-C)")
    p.add_argument("--seed", type=int, default=None)
    args = p.parse_args(argv)
    if args.nodes < 1:
        p.error("--nodes must be at least 1")
    if not 10 <= args.period_ms <= 60000:
        p.error("--period-ms must be in [10, 60000]")
    if not 0.0 <= args.loss <= 1.0:
        p.error("--loss must be in [0, 1]")
    if args.fault_rate < 0:
        p.error("--fault-rate must be >= 0")
    return args


def main(argv: list[str] | None = None) -> None:
    args = parse_args(argv)
    host, _, port = args.target.rpartition(":")
    cfg = FleetConfig(
        nodes=args.nodes,
        prefix=args.prefix,
        fault_rate=args.fault_rate,
        seed=args.seed,
        node=NodeConfig(
            period_s=args.period_ms / 1000,
            recovery_cycles=args.recovery_cycles,
            loss=args.loss,
            release_after_s=None if args.release_after < 0 else args.release_after,
        ),
    )
    run(cfg, (host or "127.0.0.1", int(port)), duration_s=args.duration)


if __name__ == "__main__":
    main()
