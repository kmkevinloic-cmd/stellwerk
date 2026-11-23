#include "stellwerk/degradation.hpp"

#include <algorithm>

namespace stellwerk {

DegradationManager::DegradationManager(std::uint32_t recovery_cycles)
    : recovery_cycles_(std::max<std::uint32_t>(1, recovery_cycles)) {}

Mode DegradationManager::evaluate(bool any_critical, bool any_degrading) noexcept {
    // Rule 1: a critical fault wins from any mode.
    if (any_critical) {
        mode_ = Mode::MinimalRisk;
        healthy_ = 0;
        return mode_;
    }
    // Rule 2: MINIMAL_RISK is latched until release().
    if (mode_ == Mode::MinimalRisk) {
        return mode_;
    }
    // Rule 3: any degrading fault means at least DEGRADED.
    if (any_degrading) {
        mode_ = Mode::Degraded;
        healthy_ = 0;
        return mode_;
    }
    // Rule 4: hysteresis on recovery.
    if (mode_ == Mode::Degraded && ++healthy_ >= recovery_cycles_) {
        mode_ = Mode::Nominal;
        healthy_ = 0;
    }
    return mode_;
}

bool DegradationManager::release(bool any_critical) noexcept {
    if (mode_ != Mode::MinimalRisk || any_critical) {
        return false;
    }
    mode_ = Mode::Degraded;
    healthy_ = 0;
    return true;
}

}  // namespace stellwerk
