#include "stellwerk/fault_monitor.hpp"

#include <gtest/gtest.h>

#include <stdexcept>

using stellwerk::DebounceConfig;
using stellwerk::FaultMonitor;
using stellwerk::Severity;

TEST(FaultMonitor, TwoFailuresConfirmWithDefaults) {
    FaultMonitor f("x", Severity::Degrade);
    f.report(true);
    EXPECT_FALSE(f.confirmed());
    EXPECT_EQ(f.fdc(), 64);
    f.report(true);
    EXPECT_TRUE(f.confirmed());
    EXPECT_EQ(f.fdc(), FaultMonitor::kFdcMax);
    EXPECT_EQ(f.occurrences(), 1u);
}

TEST(FaultMonitor, EightPassesHealWithDefaults) {
    FaultMonitor f("x", Severity::Degrade);
    f.report(true);
    f.report(true);
    for (int i = 0; i < 7; ++i) {
        f.report(false);
        EXPECT_TRUE(f.confirmed()) << "healed too early after " << i + 1 << " passes";
    }
    f.report(false);
    EXPECT_FALSE(f.confirmed());
    EXPECT_EQ(f.fdc(), FaultMonitor::kFdcMin);
}

TEST(FaultMonitor, JumpUpIgnoresHealthyHistory) {
    FaultMonitor f("x", Severity::Degrade);
    for (int i = 0; i < 100; ++i) f.report(false);
    ASSERT_EQ(f.fdc(), FaultMonitor::kFdcMin);
    f.report(true);
    EXPECT_EQ(f.fdc(), 64);
    f.report(true);
    EXPECT_TRUE(f.confirmed());
}

TEST(FaultMonitor, AlternatingNoiseNeverConfirms) {
    FaultMonitor f("x", Severity::Degrade, DebounceConfig{32, 32});
    for (int i = 0; i < 1000; ++i) f.report(i % 2 == 0);
    EXPECT_FALSE(f.confirmed());
}

TEST(FaultMonitor, ConfirmedFaultStaysUntilFullyHealed) {
    FaultMonitor f("x", Severity::MinimalRisk);
    f.report(true);
    f.report(true);
    f.report(false);  // 95
    f.report(true);   // back to 127, still the same occurrence
    EXPECT_TRUE(f.confirmed());
    EXPECT_EQ(f.occurrences(), 1u);
    for (int i = 0; i < 8; ++i) f.report(false);
    f.report(true);
    f.report(true);
    EXPECT_EQ(f.occurrences(), 2u);
}

TEST(FaultMonitor, RejectsInvalidConfig) {
    EXPECT_THROW(FaultMonitor("", Severity::Degrade), std::invalid_argument);
    EXPECT_THROW(FaultMonitor("x", Severity::Degrade, DebounceConfig{0, 32}), std::invalid_argument);
    EXPECT_THROW(FaultMonitor("x", Severity::Degrade, DebounceConfig{64, 256}), std::invalid_argument);
}
