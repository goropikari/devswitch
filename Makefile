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
	curl -L https://github.com/traefik/traefik/releases/download/v3.6.10/traefik_v3.6.10_linux_amd64.tar.gz | sudo tar xz -C /usr/local/bin traefik
