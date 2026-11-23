cmake_minimum_required(VERSION 3.16)
project(stellwerk_edge VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

option(STELLWERK_BUILD_TESTS "Build unit tests (fetches GoogleTest)" ON)
option(STELLWERK_BUILD_EXAMPLES "Build the example edge node" ON)

add_library(stellwerk_edge
  src/fault_monitor.cpp
  src/degradation.cpp
  src/supervisor.cpp
  src/heartbeat.cpp
)
add_library(stellwerk::edge ALIAS stellwerk_edge)
target_include_directories(stellwerk_edge PUBLIC
  $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
  $<INSTALL_INTERFACE:include>
)
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
  target_compile_options(stellwerk_edge PRIVATE -Wall -Wextra -Wpedantic -Wconversion -Werror)
endif()

if(STELLWERK_BUILD_EXAMPLES)
  find_package(Threads REQUIRED)
  add_executable(edge_node examples/edge_node.cpp)
  target_link_libraries(edge_node PRIVATE stellwerk::edge Threads::Threads)
endif()

if(STELLWERK_BUILD_TESTS)
  enable_testing()
  include(FetchContent)
  FetchContent_Declare(googletest
    GIT_REPOSITORY https://github.com/google/googletest.git
    GIT_TAG v1.15.2
    GIT_SHALLOW TRUE
  )
  set(INSTALL_GTEST OFF CACHE BOOL "" FORCE)
  set(gtest_force_shared_crt ON CACHE BOOL "" FORCE)
  FetchContent_MakeAvailable(googletest)

  add_executable(stellwerk_edge_tests
    tests/fault_monitor_test.cpp
    tests/degradation_test.cpp
    tests/supervisor_test.cpp
    tests/heartbeat_test.cpp
  )
  target_link_libraries(stellwerk_edge_tests PRIVATE stellwerk::edge GTest::gtest_main)
  include(GoogleTest)
  gtest_discover_tests(stellwerk_edge_tests)
endif()
