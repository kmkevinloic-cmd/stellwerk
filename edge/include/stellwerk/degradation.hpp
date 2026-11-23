#pragma once

#include <cstdint>

#include "stellwerk/types.hpp"

namespace stellwerk {

/// Owns the node mode and applies the transition rules from
/// docs/SAFETY_CONCEPT.md. It knows nothing about individual faults, only
/// whether any critical or degrading fault is currently confirmed.
class DegradationManager {
public:
    /// recovery_cycles: consecutive fault-free cycles needed to go from
    /// DEGRADED back to NOMINAL (at least 1).
    explicit DegradationManager(std::uint32_t recovery_cycles = 20);

    /// Evaluate once per supervision cycle. Returns the new mode.
    Mode evaluate(bool any_critical, bool any_degrading) noexcept;

    /// Leave the latched MINIMAL_RISK mode. Refused (returns false) when the
    /// node is not in MINIMAL_RISK or a critical fault is still confirmed.
    /// On success the node goes to DEGRADED and must recover from there.
    bool release(bool any_critical) noexcept;

    [[nodiscard]] Mode mode() const noexcept { return mode_; }
    [[nodiscard]] std::uint32_t healthy_cycles() const noexcept { return healthy_; }

private:
    std::uint32_t recovery_cycles_;
    Mode mode_ = Mode::Nominal;
    std::uint32_t healthy_ = 0;
};

}  // namespace stellwerk
