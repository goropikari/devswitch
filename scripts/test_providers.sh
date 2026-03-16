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

providers=("${@:-native}")
if [[ ${#providers[@]} -eq 1 && "${providers[0]}" == "native" ]]; then
  providers=(native)
fi

cleanup() {
  "$DEVSWITCH_BIN" cleanup >/dev/null 2>&1 || true
  "$DEVSWITCH_BIN" proxy stop >/dev/null 2>&1 || true
}

get_free_port() {
  local port_file
  port_file=$(mktemp --suffix=.go)
  cat > "$port_file" <<EOF
package main
import ("fmt"; "net"; "os")
func main() {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(l.Addr().(*net.TCPAddr).Port)
	l.Close()
}
EOF
  go run "$port_file"
  rm "$port_file"
}

wait_http_ready() {
  local port="$1"
  local retries=30
  local i
  for ((i=1; i<=retries; i++)); do
    if curl -fsS "http://localhost:${port}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.3
  done
  return 1
}

wait_grpc_ready() {
  local port="$1"
  local retries=30
  local i
  for ((i=1; i<=retries; i++)); do
    if grpcurl -plaintext "localhost:${port}" list >/dev/null 2>&1; then
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
  local port
  port=$(get_free_port)
  local external_port
  local external_pid=""


  echo "== provider: $p (port: $port) =="
  cleanup

  "$DEVSWITCH_BIN" proxy start --provider "$p" --port "$port" --ui=false >/dev/null

  "$DEVSWITCH_BIN" app start --port-arg --port --label "${p}-http-${suffix}" -- go run ./sample/server/http/main.go >/dev/null
  if ! wait_http_ready "$port"; then
    echo "FAIL: HTTP check failed for provider=$p" >&2
    return 1
  fi
  http_body="$(curl -fsS "http://localhost:${port}")"
  if [[ "$http_body" != *"Hello, World!"* ]]; then
    echo "FAIL: unexpected HTTP response for provider=$p: $http_body" >&2
    return 1
  fi
  echo "OK: HTTP"

  "$DEVSWITCH_BIN" app start --port-arg --port --label "${p}-grpc-${suffix}" -- go -C ./sample/server/grpc run main.go >/dev/null
  if ! wait_grpc_ready "$port"; then
    echo "FAIL: gRPC list check failed for provider=$p" >&2
    return 1
  fi
  grpc_reply="$(grpcurl -plaintext -d '{}' "localhost:${port}" hello.HelloService/SayHello)"
  if [[ "$grpc_reply" != *"Hello from port"* ]]; then
    echo "FAIL: unexpected gRPC response for provider=$p: $grpc_reply" >&2
    return 1
  fi
  echo "OK: gRPC"

  external_port=$(get_free_port)
  go run ./sample/server/http/main.go --port "$external_port" >/dev/null 2>&1 &
  external_pid=$!

  if ! wait_http_ready "$external_port"; then
    echo "FAIL: external HTTP server did not become ready for provider=$p" >&2
    return 1
  fi

  "$DEVSWITCH_BIN" app register --port "$external_port" --label "${p}-external-${suffix}" >/dev/null
  if ! wait_http_ready "$port"; then
    echo "FAIL: external register check failed for provider=$p" >&2
    return 1
  fi
  ext_http_body="$(curl -fsS "http://localhost:${port}")"
  if [[ "$ext_http_body" != *"Hello, World!"* ]]; then
    echo "FAIL: unexpected response after external register for provider=$p: $ext_http_body" >&2
    return 1
  fi
  echo "OK: external register"

  if [[ -n "$external_pid" ]]; then
    kill "$external_pid" >/dev/null 2>&1 || true
    external_pid=""
  fi

  cleanup
  echo "PASS: provider=$p"
}

trap cleanup EXIT

go build -o devswitch ./cmd/devswitch >/dev/null

for p in "${providers[@]}"; do
  run_for_provider "$p"
done

echo "All requested provider checks completed."
