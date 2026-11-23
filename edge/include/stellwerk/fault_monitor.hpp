#pragma once

#include <cstdint>
#include <string>

#include "stellwerk/types.hpp"

namespace stellwerk {

/// Debounce parameters for one fault. With the defaults, two consecutive
/// failures confirm the fault and eight consecutive passes heal it.
struct DebounceConfig {
    int fail_step = 64;  ///< added per failed report, 1..255
    int pass_step = 32;  ///< subtracted per passed report, 1..255
};

/// Turns noisy pass/fail reports into a confirmed fault, using a fault
/// detection counter (FDC) in [-128, 127] with "jump up" on the first failure
/// after a healthy period. Confirmed at +127, healed at -128.
class FaultMonitor {
public:
    static constexpr int kFdcMax = 127;
    static constexpr int kFdcMin = -128;

    /// Throws std::invalid_argument for empty names or out-of-range steps.
    FaultMonitor(std::string name, Severity severity, DebounceConfig cfg = {});

    void report(bool failed) noexcept;

    [[nodiscard]] bool confirmed() const noexcept { return confirmed_; }
    [[nodiscard]] int fdc() const noexcept { return fdc_; }
    [[nodiscard]] Severity severity() const noexcept { return severity_; }
    [[nodiscard]] const std::string& name() const noexcept { return name_; }
    /// Number of times this fault has been confirmed since construction.
    [[nodiscard]] std::uint32_t occurrences() const noexcept { return occurrences_; }

private:
    std::string name_;
    Severity severity_;
    DebounceConfig cfg_;
    int fdc_ = 0;
    bool confirmed_ = false;
    std::uint32_t occurrences_ = 0;
};

}  // namespace stellwerk
