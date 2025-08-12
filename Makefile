# List of sources and their target binaries
SRC_BIN_PAIRS = \
	examples/helloworld/main.go bin/helloworld \
	examples/middleware/hooks/main.go bin/hooks \
	examples/childprocs/main.go bin/childprocs \
	examples/request/main.go bin/request \
	examples/restarts/main.go bin/restarts \
	examples/eventstream/main.go bin/eventstream \
	examples/tcpserver/main.go bin/tcpserver \
	examples/metrics/main.go bin/metrics \
	examples/chat/server/main.go bin/chatserver \
	examples/chat/client/main.go bin/chatclient \
	examples/cluster/member_1/main.go bin/cluster_member_1 \
	examples/cluster/member_2/main.go bin/cluster_member_2

# Extract binaries from SRC_BIN_PAIRS
BINARIES = $(wordlist 2,2,$(SRC_BIN_PAIRS)) \
	$(wordlist 4,4,$(SRC_BIN_PAIRS)) \
	$(wordlist 6,6,$(SRC_BIN_PAIRS)) \
	$(wordlist 8,8,$(SRC_BIN_PAIRS)) \
	$(wordlist 10,10,$(SRC_BIN_PAIRS)) \
	$(wordlist 12,12,$(SRC_BIN_PAIRS)) \
	$(wordlist 14,14,$(SRC_BIN_PAIRS)) \
	$(wordlist 16,16,$(SRC_BIN_PAIRS)) \
	$(wordlist 18,18,$(SRC_BIN_PAIRS)) \
	$(wordlist 20,20,$(SRC_BIN_PAIRS)) \
	$(wordlist 22,22,$(SRC_BIN_PAIRS)) \
	$(wordlist 24,24,$(SRC_BIN_PAIRS))

.DEFAULT_GOAL := build

# Pattern rule for building binaries from sources
bin/%: examples/%/main.go
	@mkdir -p bin
	go build -o $@ $<

build: $(BINARIES)

test: build
	go test ./... -count=1 --race --timeout=5s

proto:
	@command -v protoc >/dev/null || { echo 'protoc not found'; exit 1; }
	protoc --go_out=. --go-vtproto_out=. --go_opt=paths=source_relative --proto_path=. actor/actor.proto

bench:
	go run ./_bench/.

bench-profile:
	go test -bench='^BenchmarkGoactors$$' -run=NONE -cpuprofile cpu.prof -memprofile mem.prof ./_bench

clean:
	rm -rf bin/* cpu.prof mem.prof

.PHONY: build test proto bench bench-profile clean
