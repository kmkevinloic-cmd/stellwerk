#pragma once

#include <cstddef>
#include <string>
#include <vector>

#include "stellwerk/degradation.hpp"
#include "stellwerk/fault_monitor.hpp"
#include "stellwerk/types.hpp"

namespace stellwerk {

/// Index of a fault registered with a Supervisor.
struct FaultId {
    std::size_t value;
};

/// Index of a watchdog registered with a Supervisor.
struct WatchdogId {
    std::size_t value;
};

/// Ties fault monitors, deadline watchdogs and the degradation manager
/// together. Register everything at start-up, then from the application:
///
///   * call kick() whenever a watched pipeline stage produces output,
///   * call report() for event checks such as an inference latency budget,
///   * call cycle() once per supervision period.
///
/// Not thread-safe: drive it from one thread, or guard it externally.
class Supervisor {
public:
    explicit Supervisor(std::uint32_t recovery_cycles = 20) : manager_(recovery_cycles) {}

    FaultId add_fault(std::string name, Severity severity, DebounceConfig cfg = {});

    /// The watchdog reports to `fault` every cycle: failed when more than
    /// `deadline` has passed since the last kick (or since `now` here).
    WatchdogId add_watchdog(FaultId fault, Duration deadline, TimePoint now);

    void kick(WatchdogId id, TimePoint now);
    void report(FaultId id, bool failed);

    /// Checks all watchdogs, then evaluates the mode. Returns the new mode.
    Mode cycle(TimePoint now);

    /// See DegradationManager::release.
    bool release();

    [[nodiscard]] Mode mode() const noexcept { return manager_.mode(); }
    [[nodiscard]] std::vector<std::string> confirmed_faults() const;
    [[nodiscard]] const FaultMonitor& fault(FaultId id) const { return faults_.at(id.value); }

private:
    struct Watchdog {
        FaultId fault;
        Duration deadline;
        TimePoint last_kick;
    };

    [[nodiscard]] bool any_confirmed(Severity s) const noexcept;

    std::vector<FaultMonitor> faults_;
    std::vector<Watchdog> watchdogs_;
    DegradationManager manager_;
};

}  // namespace stellwerk
