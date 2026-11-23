#include "stellwerk/supervisor.hpp"

#include <gtest/gtest.h>

#include <stdexcept>

using namespace std::chrono_literals;
using stellwerk::Mode;
using stellwerk::Severity;
using stellwerk::Supervisor;
using stellwerk::TimePoint;

namespace {
const TimePoint t0{};
}

TEST(Supervisor, WatchdogExpiryLeadsToMinimalRisk) {
    Supervisor s(3);
    const auto f = s.add_fault("camera_stall", Severity::MinimalRisk);
    const auto wd = s.add_watchdog(f, 300ms, t0);

    // Kicked every 100 ms: healthy.
    for (int i = 1; i <= 10; ++i) {
        s.kick(wd, t0 + i * 100ms);
        EXPECT_EQ(s.cycle(t0 + i * 100ms), Mode::Nominal);
    }
    // Stalls after 1000 ms. At 1300 ms elapsed == deadline: still fine.
    EXPECT_EQ(s.cycle(t0 + 1300ms), Mode::Nominal);
    // Two expired cycles confirm the fault.
    EXPECT_EQ(s.cycle(t0 + 1400ms), Mode::Nominal);
    EXPECT_EQ(s.cycle(t0 + 1500ms), Mode::MinimalRisk);
    EXPECT_EQ(s.confirmed_faults(), std::vector<std::string>{"camera_stall"});
}

TEST(Supervisor, MinimalRiskNeedsHealedFaultAndRelease) {
    Supervisor s(2);
    const auto f = s.add_fault("camera_stall", Severity::MinimalRisk);
    const auto wd = s.add_watchdog(f, 100ms, t0);
    s.cycle(t0 + 200ms);
    ASSERT_EQ(s.cycle(t0 + 300ms), Mode::MinimalRisk);

    // Camera recovers, but the fault is still confirmed until healed.
    auto now = t0 + 300ms;
    s.kick(wd, now);
    EXPECT_FALSE(s.release());
    for (int i = 0; i < 8; ++i) {
        now += 50ms;
        s.kick(wd, now);
        EXPECT_EQ(s.cycle(now), Mode::MinimalRisk);  // latched
    }
    EXPECT_TRUE(s.confirmed_faults().empty());
    EXPECT_TRUE(s.release());
    EXPECT_EQ(s.mode(), Mode::Degraded);
    now += 50ms;
    s.kick(wd, now);
    s.cycle(now);
    now += 50ms;
    s.kick(wd, now);
    EXPECT_EQ(s.cycle(now), Mode::Nominal);
}

TEST(Supervisor, LatencyReportsDegradeAndRecover) {
    Supervisor s(5);
    const auto infer = s.add_fault("infer_deadline", Severity::Degrade);
    auto now = t0;
    for (int i = 0; i < 2; ++i) {
        s.report(infer, true);
        now += 100ms;
        s.cycle(now);
    }
    EXPECT_EQ(s.mode(), Mode::Degraded);
    // The 8th pass heals the fault and that same cycle is the first of the 5
    // healthy recovery cycles, so NOMINAL is reached on cycle 8 + 5 - 1 = 12.
    int cycles = 0;
    while (s.mode() != Mode::Nominal && cycles < 100) {
        s.report(infer, false);
        now += 100ms;
        s.cycle(now);
        ++cycles;
    }
    EXPECT_EQ(cycles, 12);
}

TEST(Supervisor, CriticalOverridesDegrade) {
    Supervisor s(5);
    const auto a = s.add_fault("a", Severity::Degrade);
    const auto b = s.add_fault("b", Severity::MinimalRisk);
    for (int i = 0; i < 2; ++i) {
        s.report(a, true);
        s.report(b, true);
    }
    EXPECT_EQ(s.cycle(t0), Mode::MinimalRisk);
    EXPECT_EQ(s.confirmed_faults().size(), 2u);
}

TEST(Supervisor, RegistrationErrors) {
    Supervisor s;
    const auto f = s.add_fault("x", Severity::Degrade);
    EXPECT_THROW(s.add_fault("x", Severity::Degrade), std::invalid_argument);
    EXPECT_THROW(s.add_watchdog(stellwerk::FaultId{7}, 1ms, t0), std::out_of_range);
    EXPECT_THROW(s.add_watchdog(f, 0ms, t0), std::invalid_argument);
}
