// A simulated edge node: a camera stage and an inference stage supervised by
// Stellwerk, publishing heartbeats to stellwerkd.
//
//   ./edge_node --node-id amr-0001 --latency-fault-at 5 --camera-stall-at 15
//
// Send SIGUSR1 to request release from MINIMAL_RISK.

#include <atomic>
#include <csignal>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <random>
#include <string>
#include <thread>

#include "stellwerk/heartbeat.hpp"
#include "stellwerk/supervisor.hpp"

using namespace std::chrono_literals;
using stellwerk::Mode;
using stellwerk::Severity;

namespace {

std::atomic<bool> g_stop{false};
std::atomic<bool> g_release{false};

void on_signal(int sig) {
    if (sig == SIGUSR1) {
        g_release = true;
    } else {
        g_stop = true;
    }
}

struct Options {
    std::string node_id = "edge-0001";
    std::string host = "127.0.0.1";
    std::uint16_t port = 7400;
    int period_ms = 100;
    double duration_s = 0;           // 0 = run until interrupted
    double latency_fault_at = -1;    // seconds; < 0 = never
    double camera_stall_at = -1;
    double fault_for = 3;            // seconds each injected fault lasts
};

[[noreturn]] void usage(const char* prog) {
    std::fprintf(stderr,
                 "usage: %s [--node-id ID] [--host H] [--port P] [--period-ms N]\n"
                 "          [--duration S] [--latency-fault-at S] [--camera-stall-at S]\n"
                 "          [--fault-for S]\n",
                 prog);
    std::exit(2);
}

Options parse(int argc, char** argv) {
    Options o;
    for (int i = 1; i < argc; ++i) {
        const std::string a = argv[i];
        auto next = [&]() -> const char* {
            if (i + 1 >= argc) usage(argv[0]);
            return argv[++i];
        };
        if (a == "--node-id") o.node_id = next();
        else if (a == "--host") o.host = next();
        else if (a == "--port") o.port = static_cast<std::uint16_t>(std::atoi(next()));
        else if (a == "--period-ms") o.period_ms = std::atoi(next());
        else if (a == "--duration") o.duration_s = std::atof(next());
        else if (a == "--latency-fault-at") o.latency_fault_at = std::atof(next());
        else if (a == "--camera-stall-at") o.camera_stall_at = std::atof(next());
        else if (a == "--fault-for") o.fault_for = std::atof(next());
        else usage(argv[0]);
    }
    if (o.period_ms < 10 || o.period_ms > 60000) usage(argv[0]);
    return o;
}

bool active(double start, double length, double t) { return start >= 0 && t >= start && t < start + length; }

}  // namespace

int main(int argc, char** argv) {
    const Options opt = parse(argc, argv);
    std::signal(SIGINT, on_signal);
    std::signal(SIGTERM, on_signal);
    std::signal(SIGUSR1, on_signal);

    const auto t0 = stellwerk::Clock::now();
    stellwerk::Supervisor sup(/*recovery_cycles=*/20);
    const auto camera_fault = sup.add_fault("camera_stall", Severity::MinimalRisk);
    const auto infer_fault = sup.add_fault("infer_deadline", Severity::Degrade);
    const auto camera_wd = sup.add_watchdog(camera_fault, 300ms, t0);

    stellwerk::HeartbeatPublisher pub(opt.host, opt.port);
    stellwerk::Heartbeat hb;
    hb.node_id = opt.node_id;
    hb.boot_id = stellwerk::make_boot_id();
    hb.period_ms = static_cast<std::uint32_t>(opt.period_ms);

    std::mt19937 rng(std::random_device{}());
    std::normal_distribution<double> nominal_latency(22.0, 4.0);
    constexpr double kBudgetMs = 50.0;

    std::printf("node %s boot %s -> %s:%u\n", hb.node_id.c_str(), hb.boot_id.c_str(), opt.host.c_str(),
                static_cast<unsigned>(opt.port));
    std::fflush(stdout);

    Mode last = sup.mode();
    auto next = t0;
    while (!g_stop) {
        const auto now = stellwerk::Clock::now();
        const double t = std::chrono::duration<double>(now - t0).count();
        if (opt.duration_s > 0 && t >= opt.duration_s) break;

        // Camera stage: produces frames unless a stall is injected.
        if (!active(opt.camera_stall_at, opt.fault_for, t)) sup.kick(camera_wd, now);

        // Inference stage: latency check against the budget.
        double latency = nominal_latency(rng);
        if (active(opt.latency_fault_at, opt.fault_for, t)) latency += 60.0;
        sup.report(infer_fault, latency > kBudgetMs);

        if (g_release.exchange(false)) {
            std::printf("[%7.2fs] release requested: %s\n", t, sup.release() ? "accepted" : "refused");
        }

        const Mode mode = sup.cycle(now);
        if (mode != last) {
            std::printf("[%7.2fs] mode %s -> %s\n", t, std::string(to_string(last)).c_str(),
                        std::string(to_string(mode)).c_str());
            last = mode;
        }

        hb.mode = mode;
        hb.uptime_ms = static_cast<std::uint64_t>(t * 1000.0);
        hb.faults = sup.confirmed_faults();
        hb.metrics = {{"infer_ms", latency}, {"camera_fdc", sup.fault(camera_fault).fdc()}};
        pub.publish(hb);
        ++hb.seq;
        std::fflush(stdout);

        next += std::chrono::milliseconds(opt.period_ms);
        std::this_thread::sleep_until(next);
    }
    std::printf("stopped after %llu heartbeats (%llu failed)\n", static_cast<unsigned long long>(pub.sent()),
                static_cast<unsigned long long>(pub.failed()));
    return 0;
}
