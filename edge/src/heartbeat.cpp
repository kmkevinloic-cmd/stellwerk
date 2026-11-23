#include "stellwerk/heartbeat.hpp"

#include <netdb.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cerrno>
#include <cmath>
#include <cstdio>
#include <cstring>
#include <random>
#include <stdexcept>

namespace stellwerk {
namespace {

void append_string(std::string& out, const std::string& s) {
    out.push_back('"');
    for (const char c : s) {
        switch (c) {
            case '"': out += "\\\""; break;
            case '\\': out += "\\\\"; break;
            case '\n': out += "\\n"; break;
            case '\r': out += "\\r"; break;
            case '\t': out += "\\t"; break;
            default:
                if (static_cast<unsigned char>(c) < 0x20) {
                    char buf[8];
                    std::snprintf(buf, sizeof buf, "\\u%04x", static_cast<unsigned>(c));
                    out += buf;
                } else {
                    out.push_back(c);
                }
        }
    }
    out.push_back('"');
}

void append_double(std::string& out, double v) {
    char buf[32];
    std::snprintf(buf, sizeof buf, "%.6g", v);
    out += buf;
}

}  // namespace

std::string encode(const Heartbeat& hb) {
    std::string out;
    out.reserve(256);
    out += "{\"v\":1,\"node_id\":";
    append_string(out, hb.node_id);
    out += ",\"boot_id\":";
    append_string(out, hb.boot_id);
    out += ",\"seq\":" + std::to_string(hb.seq);
    out += ",\"period_ms\":" + std::to_string(hb.period_ms);
    out += ",\"uptime_ms\":" + std::to_string(hb.uptime_ms);
    out += ",\"mode\":\"";
    out += to_string(hb.mode);
    out += "\",\"faults\":[";
    for (std::size_t i = 0; i < hb.faults.size(); ++i) {
        if (i > 0) out.push_back(',');
        append_string(out, hb.faults[i]);
    }
    out += "],\"metrics\":{";
    bool first = true;
    for (const auto& [key, value] : hb.metrics) {
        if (!std::isfinite(value)) continue;
        if (!first) out.push_back(',');
        first = false;
        append_string(out, key);
        out.push_back(':');
        append_double(out, value);
    }
    out += "}}";
    return out;
}

std::string make_boot_id() {
    std::random_device rd;
    std::uniform_int_distribution<int> nibble(0, 15);
    static constexpr char kHex[] = "0123456789abcdef";
    std::string id(16, '0');
    for (auto& c : id) c = kHex[nibble(rd)];
    return id;
}

HeartbeatPublisher::HeartbeatPublisher(const std::string& host, std::uint16_t port) {
    addrinfo hints{};
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_DGRAM;
    addrinfo* res = nullptr;
    const std::string service = std::to_string(port);
    if (const int rc = ::getaddrinfo(host.c_str(), service.c_str(), &hints, &res); rc != 0) {
        throw std::runtime_error("resolve " + host + ": " + ::gai_strerror(rc));
    }
    fd_ = ::socket(res->ai_family, res->ai_socktype, res->ai_protocol);
    if (fd_ < 0) {
        ::freeaddrinfo(res);
        throw std::runtime_error(std::string("socket: ") + std::strerror(errno));
    }
    const auto* p = reinterpret_cast<const unsigned char*>(res->ai_addr);
    addr_.assign(p, p + res->ai_addrlen);
    ::freeaddrinfo(res);
}

HeartbeatPublisher::~HeartbeatPublisher() {
    if (fd_ >= 0) ::close(fd_);
}

bool HeartbeatPublisher::publish(const Heartbeat& hb) noexcept {
    try {
        const std::string msg = encode(hb);
        if (msg.size() > kMaxDatagramBytes) {
            ++failed_;
            return false;
        }
        const auto n = ::sendto(fd_, msg.data(), msg.size(), MSG_DONTWAIT,
                                reinterpret_cast<const sockaddr*>(addr_.data()),
                                static_cast<socklen_t>(addr_.size()));
        if (n != static_cast<ssize_t>(msg.size())) {
            ++failed_;
            return false;
        }
        ++sent_;
        return true;
    } catch (...) {  // allocation failure while encoding
        ++failed_;
        return false;
    }
}

}  // namespace stellwerk
