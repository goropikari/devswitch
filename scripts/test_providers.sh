#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

if [[ -x "$ROOT_DIR/devswitch" ]]; then
  DEVSWITCH_BIN="$ROOT_DIR/devswitch"
else
  DEVSWITCH_BIN="devswitch"
fi

required_cmds=(curl go)
for cmd in "${required_cmds[@]}"; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $cmd" >&2
    exit 1
  fi
done

if ! command -v "$DEVSWITCH_BIN" >/dev/null 2>&1; then
  echo "ERROR: devswitch command not found. Build first: go build -o devswitch ./cmd/devswitch" >&2
  exit 1
fi

if ! command -v grpcurl >/dev/null 2>&1; then
  echo "ERROR: grpcurl is required for gRPC checks" >&2
  exit 1
fi

providers=("${@:-native traefik socat}")
if [[ ${#providers[@]} -eq 1 && "${providers[0]}" == "native traefik socat" ]]; then
  providers=(native traefik socat)
fi

cleanup() {
  "$DEVSWITCH_BIN" cleanup >/dev/null 2>&1 || true
  "$DEVSWITCH_BIN" proxy stop >/dev/null 2>&1 || true
}

wait_http_ready() {
  local retries=30
  local i
  for ((i=1; i<=retries; i++)); do
    if curl -fsS "http://localhost:9000" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.3
  done
  return 1
}

wait_grpc_ready() {
  local retries=30
  local i
  for ((i=1; i<=retries; i++)); do
    if grpcurl -plaintext localhost:9000 list >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.3
  done
  return 1
}

run_for_provider() {
  local p="$1"
  local suffix
  suffix="$(date +%s)-$$"

  if [[ "$p" == "traefik" ]] && ! command -v traefik >/dev/null 2>&1; then
    echo "SKIP: provider=traefik (traefik command not found)"
    return 0
  fi
  if [[ "$p" == "socat" ]] && ! command -v socat >/dev/null 2>&1; then
    echo "SKIP: provider=socat (socat command not found)"
    return 0
  fi

  echo "== provider: $p =="
  cleanup

  "$DEVSWITCH_BIN" proxy start --provider "$p" >/dev/null

  "$DEVSWITCH_BIN" app start --port-arg --port --label "${p}-http-${suffix}" -- go run ./sample/server/http/main.go >/dev/null
  if ! wait_http_ready; then
    echo "FAIL: HTTP check failed for provider=$p" >&2
    return 1
  fi
  http_body="$(curl -fsS "http://localhost:9000")"
  if [[ "$http_body" != *"Hello, World!"* ]]; then
    echo "FAIL: unexpected HTTP response for provider=$p: $http_body" >&2
    return 1
  fi
  echo "OK: HTTP"

  "$DEVSWITCH_BIN" app start --grpc --port-arg --port --label "${p}-grpc-${suffix}" -- go -C ./sample/server/grpc run main.go >/dev/null
  if ! wait_grpc_ready; then
    echo "FAIL: gRPC list check failed for provider=$p" >&2
    return 1
  fi
  grpc_reply="$(grpcurl -plaintext -d '{}' localhost:9000 hello.HelloService/SayHello)"
  if [[ "$grpc_reply" != *"Hello from port"* ]]; then
    echo "FAIL: unexpected gRPC response for provider=$p: $grpc_reply" >&2
    return 1
  fi
  echo "OK: gRPC"

  cleanup
  echo "PASS: provider=$p"
}

trap cleanup EXIT

go build -o devswitch ./cmd/devswitch >/dev/null

for p in "${providers[@]}"; do
  run_for_provider "$p"
done

echo "All requested provider checks completed."
