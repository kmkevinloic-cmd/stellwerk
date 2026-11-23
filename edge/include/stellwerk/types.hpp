#pragma once

#include <chrono>
#include <cstdint>
#include <string_view>

namespace stellwerk {

/// All timing in the edge library uses a monotonic clock. Every API takes
/// `now` explicitly so behaviour is deterministic and testable.
using Clock = std::chrono::steady_clock;
using TimePoint = Clock::time_point;
using Duration = Clock::duration;

/// Node operating mode. See docs/SAFETY_CONCEPT.md.
enum class Mode : std::uint8_t { Nominal, Degraded, MinimalRisk };

/// What a confirmed fault does to the node mode.
enum class Severity : std::uint8_t { Degrade, MinimalRisk };

/// Protocol name of a mode ("NOMINAL", "DEGRADED", "MINIMAL_RISK").
constexpr std::string_view to_string(Mode m) noexcept {
    switch (m) {
        case Mode::Nominal: return "NOMINAL";
        case Mode::Degraded: return "DEGRADED";
        case Mode::MinimalRisk: return "MINIMAL_RISK";
    }
    return "MINIMAL_RISK";  // unreachable; fail towards the safe side
}

}  // namespace stellwerk
