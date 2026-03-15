# devswitch

English is the primary documentation language for this repository.
Japanese docs are available in [README.ja.md](README.ja.md).

`devswitch` is a CLI tool for running multiple local development servers and
switching traffic from one stable endpoint to the active target.

It uses Traefik as a reverse proxy and rewrites dynamic config to switch
HTTP / gRPC backends instantly.

## Architecture

```
client
  |
  v
localhost:9000
  |
  v
Traefik (reverse proxy)
  |
  v
active dev server
```

What devswitch does:

- Starts app servers on free ports
- Stores started server metadata (branch + port + PID)
- Updates Traefik dynamic config
- Switches active target with an interactive selector (`promptui`)
- Shares runtime state across git worktrees in the same repository

## Requirements

- Go 1.26+
- Traefik
- interactive terminal (TTY)

Install examples:

No extra selector command is required. `devswitch` uses `promptui` internally.

Install Traefik from the official docs (Linux):
<https://doc.traefik.io/traefik/getting-started/install-traefik/>

## Installation

```bash
git clone https://github.com/goropikari/devswitch.git
cd devswitch
go build -o devswitch
```

Install with `go install`:

```bash
go install github.com/goropikari/devswitch@latest
```

## Environment Variables

| Variable | Description | Default |
| --- | --- | --- |
| `DEVSWITCH_PORT` | proxy listen port | `9000` |
| `DEVSWITCH_TMPDIR` | directory for state/log/config files | auto-generated under `/tmp` |

Example:

```bash
export DEVSWITCH_PORT=9000
export DEVSWITCH_TMPDIR=/tmp/devswitch
```

## Usage

### 1) Start proxy

```bash
devswitch proxy start
```

`proxy start` runs in daemon mode by default.

Stop proxy:

```bash
devswitch proxy stop
```

Show proxy log path:

```bash
devswitch info
```

Then access:

```
http://localhost:9000
```

### 2) Start app server

Run the HTTP sample (`http/main.go`) with `--port`:

```bash
devswitch start-server \
  --port-arg --port \
  -- go run ./http/main.go
```

### 3) gRPC mode

Start a gRPC backend with `--grpc`:

```bash
devswitch start-server \
  --grpc \
  --port-arg --port \
  -- go -C ./grpc run main.go
```

Traefik backend scheme is switched to `h2c`.

grpcurl examples (through proxy default port `9000`):

```bash
grpcurl -plaintext localhost:9000 list
grpcurl -plaintext localhost:9000 list hello.HelloService
grpcurl -plaintext -d '{}' localhost:9000 hello.HelloService/SayHello
```

### 4) Switch active server

```bash
devswitch switch
```

### 5) Manage running servers

```bash
devswitch list
devswitch stop
devswitch cleanup
```

`list` shows servers in `BRANCH PORT PID ACTIVE` order.

## Runtime Files

Files are created under `<tmpdir>`:

- `<tmpdir>/devswitch_static.yml` (Traefik static config)
- `<tmpdir>/devswitch_dynamic.yml` (routing config)
- `<tmpdir>/devswitch_servers` (server registry)
- `<tmpdir>/devswitch_active` (active target)
- `<tmpdir>/proxy.pid` (proxy daemon PID)
- `<tmpdir>/proxy.log` (proxy log)

## Devcontainer

If using devcontainer port forwarding, one port is enough:

```json
"forwardPorts": [9000]
```

## License

MIT
