#pragma once

#include <cstdint>
#include <string>
#include <utility>
#include <vector>

#include "stellwerk/types.hpp"

namespace stellwerk {

/// Contents of one heartbeat (protocol v1, docs/PROTOCOL.md).
struct Heartbeat {
    std::string node_id;
    std::string boot_id;
    std::uint64_t seq = 0;
    std::uint32_t period_ms = 100;
    std::uint64_t uptime_ms = 0;
    Mode mode = Mode::Nominal;
    std::vector<std::string> faults;
    std::vector<std::pair<std::string, double>> metrics;
};

/// Serialise to the v1 JSON wire format. Non-finite metric values are
/// skipped because JSON cannot represent them.
std::string encode(const Heartbeat& hb);

/// A new random 16-character hex identifier for this process start.
std::string make_boot_id();

/// Sends heartbeats as UDP datagrams. Sending never blocks the caller for
/// long and never throws: a lost heartbeat is replaced by the next one.
class HeartbeatPublisher {
public:
    /// host is an IPv4 address or hostname. Throws std::runtime_error if the
    /// socket cannot be created or the address cannot be resolved.
    HeartbeatPublisher(const std::string& host, std::uint16_t port);
    ~HeartbeatPublisher();

    HeartbeatPublisher(const HeartbeatPublisher&) = delete;
    HeartbeatPublisher& operator=(const HeartbeatPublisher&) = delete;

    /// Encodes hb and sends it. Returns false if the datagram was too large
    /// or the send failed.
    bool publish(const Heartbeat& hb) noexcept;

    [[nodiscard]] std::uint64_t sent() const noexcept { return sent_; }
    [[nodiscard]] std::uint64_t failed() const noexcept { return failed_; }

private:
    int fd_ = -1;
    std::vector<unsigned char> addr_;  // sockaddr storage
    std::uint64_t sent_ = 0;
    std::uint64_t failed_ = 0;
};

inline constexpr std::size_t kMaxDatagramBytes = 1400;

}  // namespace stellwerk
