#include "stellwerk/heartbeat.hpp"

#include <gtest/gtest.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cmath>
#include <limits>
#include <string>

using stellwerk::Heartbeat;
using stellwerk::Mode;

TEST(HeartbeatEncode, MatchesWireFormat) {
    Heartbeat hb;
    hb.node_id = "amr-0042";
    hb.boot_id = "abc";
    hb.seq = 7;
    hb.period_ms = 100;
    hb.uptime_ms = 700;
    hb.mode = Mode::Degraded;
    hb.faults = {"infer_deadline"};
    hb.metrics = {{"battery", 0.5}, {"nan", std::numeric_limits<double>::quiet_NaN()}, {"t", 41.75}};
    EXPECT_EQ(stellwerk::encode(hb),
              R"({"v":1,"node_id":"amr-0042","boot_id":"abc","seq":7,"period_ms":100,"uptime_ms":700,)"
              R"("mode":"DEGRADED","faults":["infer_deadline"],"metrics":{"battery":0.5,"t":41.75}})");
}

TEST(HeartbeatEncode, EscapesStrings) {
    Heartbeat hb;
    hb.node_id = "a\"b\\c\n";
    hb.boot_id = std::string("x\x01", 2);
    const std::string s = stellwerk::encode(hb);
    EXPECT_NE(s.find(R"("node_id":"a\"b\\c\n")"), std::string::npos) << s;
    EXPECT_NE(s.find(R"("boot_id":"x\u0001")"), std::string::npos) << s;
}

TEST(HeartbeatEncode, EmptyFaultsAndMetrics) {
    Heartbeat hb;
    hb.node_id = "n";
    hb.boot_id = "b";
    const std::string s = stellwerk::encode(hb);
    EXPECT_NE(s.find(R"("mode":"NOMINAL","faults":[],"metrics":{}})"), std::string::npos) << s;
}

TEST(BootId, IsSixteenHexCharsAndRandom) {
    const auto a = stellwerk::make_boot_id();
    const auto b = stellwerk::make_boot_id();
    EXPECT_EQ(a.size(), 16u);
    EXPECT_EQ(a.find_first_not_of("0123456789abcdef"), std::string::npos);
    EXPECT_NE(a, b);
}

TEST(HeartbeatPublisher, SendsDatagram) {
    const int rx = ::socket(AF_INET, SOCK_DGRAM, 0);
    ASSERT_GE(rx, 0);
    sockaddr_in addr{};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    ASSERT_EQ(::bind(rx, reinterpret_cast<sockaddr*>(&addr), sizeof addr), 0);
    socklen_t len = sizeof addr;
    ASSERT_EQ(::getsockname(rx, reinterpret_cast<sockaddr*>(&addr), &len), 0);
    timeval tv{2, 0};
    ::setsockopt(rx, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof tv);

    stellwerk::HeartbeatPublisher pub("127.0.0.1", ntohs(addr.sin_port));
    Heartbeat hb;
    hb.node_id = "n1";
    hb.boot_id = "b1";
    ASSERT_TRUE(pub.publish(hb));

    char buf[2048];
    const auto n = ::recv(rx, buf, sizeof buf, 0);
    ::close(rx);
    ASSERT_GT(n, 0);
    EXPECT_EQ(std::string(buf, static_cast<std::size_t>(n)), stellwerk::encode(hb));
    EXPECT_EQ(pub.sent(), 1u);
}

TEST(HeartbeatPublisher, RejectsOversizeDatagram) {
    stellwerk::HeartbeatPublisher pub("127.0.0.1", 9);
    Heartbeat hb;
    hb.node_id = std::string(2000, 'x');
    hb.boot_id = "b";
    EXPECT_FALSE(pub.publish(hb));
    EXPECT_EQ(pub.failed(), 1u);
}
