#include "stellwerk/supervisor.hpp"

#include <stdexcept>
#include <utility>

namespace stellwerk {

FaultId Supervisor::add_fault(std::string name, Severity severity, DebounceConfig cfg) {
    for (const auto& f : faults_) {
        if (f.name() == name) {
            throw std::invalid_argument("duplicate fault name: " + name);
        }
    }
    faults_.emplace_back(std::move(name), severity, cfg);
    return FaultId{faults_.size() - 1};
}

WatchdogId Supervisor::add_watchdog(FaultId fault, Duration deadline, TimePoint now) {
    if (fault.value >= faults_.size()) {
        throw std::out_of_range("unknown fault id");
    }
    if (deadline <= Duration::zero()) {
        throw std::invalid_argument("watchdog deadline must be positive");
    }
    watchdogs_.push_back(Watchdog{fault, deadline, now});
    return WatchdogId{watchdogs_.size() - 1};
}

void Supervisor::kick(WatchdogId id, TimePoint now) { watchdogs_.at(id.value).last_kick = now; }

void Supervisor::report(FaultId id, bool failed) { faults_.at(id.value).report(failed); }

Mode Supervisor::cycle(TimePoint now) {
    for (const auto& w : watchdogs_) {
        faults_[w.fault.value].report(now - w.last_kick > w.deadline);
    }
    return manager_.evaluate(any_confirmed(Severity::MinimalRisk), any_confirmed(Severity::Degrade));
}

bool Supervisor::release() { return manager_.release(any_confirmed(Severity::MinimalRisk)); }

std::vector<std::string> Supervisor::confirmed_faults() const {
    std::vector<std::string> out;
    for (const auto& f : faults_) {
        if (f.confirmed()) {
            out.push_back(f.name());
        }
    }
    return out;
}

bool Supervisor::any_confirmed(Severity s) const noexcept {
    for (const auto& f : faults_) {
        if (f.severity() == s && f.confirmed()) {
            return true;
        }
    }
    return false;
}

}  // namespace stellwerk
