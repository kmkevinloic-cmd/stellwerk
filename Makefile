.PHONY: all build test test-go test-edge test-sim clean

all: build

build:
	cd control-plane && go build -o bin/stellwerkd ./cmd/stellwerkd
	cmake -S edge -B edge/build -DCMAKE_BUILD_TYPE=RelWithDebInfo
	cmake --build edge/build -j

test: test-go test-edge test-sim

test-go:
	cd control-plane && go vet ./... && go test -race ./...

test-edge:
	cmake -S edge -B edge/build -DCMAKE_BUILD_TYPE=Debug
	cmake --build edge/build -j
	cd edge/build && ctest --output-on-failure

test-sim:
	cd sim && python3 -m unittest discover -s tests

clean:
	rm -rf control-plane/bin edge/build
