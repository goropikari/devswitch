.PHONY: build test-providers test-providers-native install-tool

VERSION ?= dev
LDFLAGS := -ldflags "-X github.com/goropikari/devswitch/internal/devswitch.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o devswitch ./cmd/devswitch

test-providers:
	./scripts/test_providers.sh

test-providers-native:
	./scripts/test_providers.sh native

install-tool:
	go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
	npm install -g portless
	npm install -g @google/gemini-cli
