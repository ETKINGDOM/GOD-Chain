GO ?= go
PROTOC ?= protoc
.DEFAULT_GOAL := check
GOD_BUILD_FLAGS = -mod=readonly -modfile="$(CURDIR)/build/.godcomet/god.mod" -overlay="$(CURDIR)/build/.godcomet/overlay.json"

.PHONY: prepare-consensus check build status compile-targets generate-proto

prepare-consensus:
	GOOS= GOARCH= $(GO) run -mod=readonly ./cmd/godbuild -go "$(GO)"

check build compile-targets generate-proto: prepare-consensus

check:
	$(GO) vet $(GOD_BUILD_FLAGS) -p 2 ./...
	$(GO) mod verify
	$(GO) build $(GOD_BUILD_FLAGS) -p 2 ./...

build:
	$(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/godd ./cmd/godd
	$(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/godpack ./cmd/godpack

status: build
	./build/godd status

# Synthetic candidates only. Foreign compilation is not runtime acceptance.
compile-targets:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/linux-amd64/godd ./cmd/godd
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/linux-arm64/godd ./cmd/godd
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/linux-amd64/godpack ./cmd/godpack
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/linux-arm64/godpack ./cmd/godpack
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/darwin-amd64/godd ./cmd/godd
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/darwin-arm64/godd ./cmd/godd
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(GOD_BUILD_FLAGS) -p 2 -trimpath -o build/windows-amd64/godd.exe ./cmd/godd

generate-proto:
	@test "$$($(PROTOC) --version)" = "libprotoc 33.0"
	mkdir -p build/.proto/bin
	$(GO) build $(GOD_BUILD_FLAGS) -o build/.proto/bin/protoc-gen-gocosmos github.com/cosmos/gogoproto/protoc-gen-gocosmos
	PATH="$(CURDIR)/build/.proto/bin:$$PATH" $(PROTOC) -I proto -I "$$($(GO) list $(GOD_BUILD_FLAGS) -f '{{.Module.Dir}}' github.com/cosmos/cosmos-sdk/types/msgservice)/proto" -I "$$($(GO) list $(GOD_BUILD_FLAGS) -f '{{.Module.Dir}}' github.com/cosmos/gogoproto/proto)" --gocosmos_out=plugins=grpc,paths=source_relative:build/.proto proto/godchain/godrewards/v1/tx.proto
	cp build/.proto/godchain/godrewards/v1/tx.pb.go x/godrewards/msg/tx.pb.go
	PATH="$(CURDIR)/build/.proto/bin:$$PATH" $(PROTOC) -I proto -I "$$($(GO) list $(GOD_BUILD_FLAGS) -f '{{.Module.Dir}}' github.com/cosmos/cosmos-sdk/types/msgservice)/proto" -I "$$($(GO) list $(GOD_BUILD_FLAGS) -f '{{.Module.Dir}}' github.com/cosmos/gogoproto/proto)" --gocosmos_out=plugins=grpc,paths=source_relative:build/.proto proto/godchain/godbridge/v1/tx.proto
	cp build/.proto/godchain/godbridge/v1/tx.pb.go x/godbridge/msg/tx.pb.go
	$(GO) fmt ./x/godrewards/msg ./x/godbridge/msg
