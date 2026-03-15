# devswitch

English is the primary documentation language for this repository.
Japanese docs are available in [README.ja.md](README.ja.md).

`devswitch` is a CLI tool for running multiple local development servers and
switching traffic from one stable endpoint to the active target.

It uses a reverse proxy (native/Traefik/socat) to switch HTTP / gRPC backends instantly.

## Architecture

```
client
  |
  v
localhost:9000
  |
  v
reverse proxy (native / Traefik / socat)
  |
  v
active dev server
```

What devswitch does:

- Starts app servers on free ports
- Stores started server metadata (label + branch + port + PID)
- Updates proxy routing
- Switches active target with an interactive selector (`promptui`)
- Shares runtime state across git worktrees in the same repository

## Requirements

- Go 1.26+
- interactive terminal (TTY)

Per-provider extra requirements:

| Provider           | Requirement                                                                       |
| ------------------ | --------------------------------------------------------------------------------- |
| `native` (default) | none (pure Go)                                                                    |
| `traefik`          | [Traefik binary](https://doc.traefik.io/traefik/getting-started/install-traefik/) |
| `socat`            | socat command                                                                     |

## Installation

```bash
git clone https://github.com/goropikari/devswitch.git
cd devswitch
go build -o devswitch ./cmd/devswitch
```

Install with `go install`:

```bash
go install github.com/goropikari/devswitch/cmd/devswitch@latest
```

## Environment Variables

| Variable                   | Description                                   | Default                     | Affected commands            |
| -------------------------- | --------------------------------------------- | --------------------------- | ---------------------------- |
| `DEVSWITCH_PORT`           | proxy listen port                             | `9000`                      | proxy start, app start, info |
| `DEVSWITCH_BIND_HOST`      | proxy bind host                               | `localhost`                 | proxy start, info            |
| `DEVSWITCH_PROXY_PROVIDER` | proxy provider (`native`\|`traefik`\|`socat`) | `native`                    | proxy start, info            |
| `DEVSWITCH_TMPDIR`         | directory for state/log/config files          | auto-generated under `/tmp` | all commands                 |

## Usage

### 1) Start proxy

```bash
devswitch proxy start
```

`proxy start` runs in daemon mode by default.

```bash
# change listen port (default: 9000)
# --port flag takes precedence over DEVSWITCH_PORT
devswitch proxy start --port 8080
# or
DEVSWITCH_PORT=8080 devswitch proxy start

# bind to all interfaces (for devcontainer → host access)
devswitch proxy start -b 0.0.0.0

# select provider
devswitch proxy start --provider traefik

# stop
devswitch proxy stop

# show status
devswitch info
```

Then access:

```
http://localhost:9000
```

### 2) Start app server

`devswitch app start` automatically picks a free port and passes it to the app.

```bash
# pass port via environment variable
devswitch app start --port-env PORT -- python -m http.server
# => PORT=54321 python -m http.server

# pass port via CLI flag
devswitch app start --port-arg --port -- go run ./sample/server/http/main.go
# => go run ./sample/server/http/main.go --port 54321
```

#### Labels

Assign a label to identify each process when multiple servers run on the same branch.
If omitted, a random Docker-style name (`adjective_noun`) is generated.

```bash
devswitch app start --label my-feature --port-env PORT -- ./myapp
devswitch app start -l debug-build --port-arg --port -- ./myapp
```

#### gRPC mode

```bash
devswitch app start --grpc --port-arg --port -- go -C ./sample/server/grpc run main.go
# => go -C ./sample/server/grpc run main.go --port 54321  (h2c routing)
```

grpcurl examples:

```bash
grpcurl -plaintext localhost:9000 list
grpcurl -plaintext localhost:9000 list hello.HelloService
grpcurl -plaintext -d '{}' localhost:9000 hello.HelloService/SayHello
```

### 3) Switch active server

```bash
devswitch switch
```

An interactive selector opens. Entries show label, branch, port, and command:

```
happy_turing           branch=[main]          port=54321 pid=12345 cmd=...
nervous_hopper         branch=[feature/login]  port=54322 pid=12346 cmd=...
```

### 4) List running servers

```bash
devswitch list
```

```
LABEL                  BRANCH            PORT     PID      ACTIVE CMD
happy_turing           [main]            54321    12345    *      PORT=54321 ./myapp
nervous_hopper         [feature/login]   54322    12346           PORT=54322 ./myapp
```

The active backend is marked with `*`.

### 5) Stop an app

```bash
devswitch app stop
```

If the stopped app was active, devswitch automatically switches to another running app.

### 6) Stop all and reset

```bash
devswitch cleanup
```

Terminates all registered app processes and resets registry and active state.

### 7) Show version

```bash
devswitch version
# or
devswitch --version
```

## Devcontainer

Only one port needs to be forwarded:

```json
"forwardPorts": [9000]
```

To make the proxy reachable from the host:

```bash
DEVSWITCH_BIND_HOST=0.0.0.0 devswitch proxy start
# or
devswitch proxy start -b 0.0.0.0
```

## Runtime Files

All files are created under `<tmpdir>`:

| Path                    | Purpose                                      |
| ----------------------- | -------------------------------------------- |
| `devswitch_static.yml`  | Traefik static config                        |
| `devswitch_dynamic.yml` | routing config                               |
| `devswitch_servers`     | server registry                              |
| `devswitch_active`      | active target port                           |
| `proxy.pid`             | proxy daemon PID                             |
| `proxy.log`             | proxy log (daemon mode only)                 |
| `proxy.port`            | proxy listen port persisted by `proxy start` |
| `proxy.provider`        | current provider name                        |

## AI Notice

Parts of this project were implemented with assistance from AI coding tools.

## License

MIT
