.PHONY: build test-providers test-providers-native

VERSION ?= dev
LDFLAGS := -ldflags "-X github.com/goropikari/devswitch/internal/devswitch.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o devswitch ./cmd/devswitch

test-providers:
	./scripts/test_providers.sh

test-providers-native:
	./scripts/test_providers.sh native