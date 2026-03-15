.PHONY: build test-providers test-providers-native

build:
	go build ./cmd/devswitch

test-providers:
	./scripts/test_providers.sh

test-providers-native:
	./scripts/test_providers.sh native