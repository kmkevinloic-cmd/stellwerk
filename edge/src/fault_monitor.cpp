#include "stellwerk/fault_monitor.hpp"

#include <algorithm>
#include <stdexcept>
#include <utility>

namespace stellwerk {

FaultMonitor::FaultMonitor(std::string name, Severity severity, DebounceConfig cfg)
    : name_(std::move(name)), severity_(severity), cfg_(cfg) {
    if (name_.empty()) {
        throw std::invalid_argument("fault name must not be empty");
    }
    if (cfg_.fail_step < 1 || cfg_.fail_step > 255 || cfg_.pass_step < 1 || cfg_.pass_step > 255) {
        throw std::invalid_argument("debounce steps must be in [1, 255]");
    }
}

void FaultMonitor::report(bool failed) noexcept {
    if (failed) {
        // Jump up: a long healthy history must not delay detection.
        fdc_ = std::min(kFdcMax, std::max(fdc_, 0) + cfg_.fail_step);
        if (fdc_ == kFdcMax && !confirmed_) {
            confirmed_ = true;
            ++occurrences_;
        }
    } else {
        fdc_ = std::max(kFdcMin, fdc_ - cfg_.pass_step);
        if (fdc_ == kFdcMin) {
            confirmed_ = false;
        }
    }
}

}  // namespace stellwerk
