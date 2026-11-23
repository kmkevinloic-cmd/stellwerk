#include "stellwerk/degradation.hpp"

#include <gtest/gtest.h>

using stellwerk::DegradationManager;
using stellwerk::Mode;

TEST(Degradation, StartsNominal) {
    DegradationManager m(5);
    EXPECT_EQ(m.evaluate(false, false), Mode::Nominal);
}

TEST(Degradation, DegradingFaultDegrades) {
    DegradationManager m(5);
    EXPECT_EQ(m.evaluate(false, true), Mode::Degraded);
}

TEST(Degradation, RecoveryNeedsConsecutiveHealthyCycles) {
    DegradationManager m(5);
    m.evaluate(false, true);
    for (int i = 0; i < 4; ++i) EXPECT_EQ(m.evaluate(false, false), Mode::Degraded);
    // A fault inside the window resets the count.
    m.evaluate(false, true);
    for (int i = 0; i < 4; ++i) EXPECT_EQ(m.evaluate(false, false), Mode::Degraded);
    EXPECT_EQ(m.evaluate(false, false), Mode::Nominal);
}

TEST(Degradation, CriticalFaultWinsFromAnyMode) {
    DegradationManager a(5);
    EXPECT_EQ(a.evaluate(true, false), Mode::MinimalRisk);
    DegradationManager b(5);
    b.evaluate(false, true);
    EXPECT_EQ(b.evaluate(true, true), Mode::MinimalRisk);
}

TEST(Degradation, MinimalRiskIsLatched) {
    DegradationManager m(1);
    m.evaluate(true, false);
    for (int i = 0; i < 100; ++i) EXPECT_EQ(m.evaluate(false, false), Mode::MinimalRisk);
}

TEST(Degradation, ReleaseRefusedWhileCriticalFaultActive) {
    DegradationManager m(1);
    m.evaluate(true, false);
    EXPECT_FALSE(m.release(true));
    EXPECT_EQ(m.mode(), Mode::MinimalRisk);
}

TEST(Degradation, ReleaseGoesToDegradedThenRecovers) {
    DegradationManager m(3);
    m.evaluate(true, false);
    ASSERT_TRUE(m.release(false));
    EXPECT_EQ(m.mode(), Mode::Degraded);
    EXPECT_EQ(m.evaluate(false, false), Mode::Degraded);
    EXPECT_EQ(m.evaluate(false, false), Mode::Degraded);
    EXPECT_EQ(m.evaluate(false, false), Mode::Nominal);
}

TEST(Degradation, ReleaseOutsideMinimalRiskIsRefused) {
    DegradationManager m(3);
    EXPECT_FALSE(m.release(false));
    m.evaluate(false, true);
    EXPECT_FALSE(m.release(false));
    EXPECT_EQ(m.mode(), Mode::Degraded);
}

TEST(Degradation, ZeroRecoveryCyclesIsClampedToOne) {
    DegradationManager m(0);
    m.evaluate(false, true);
    EXPECT_EQ(m.evaluate(false, false), Mode::Nominal);
}
