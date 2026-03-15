# devswitch Architecture

## Purpose

`devswitch` is a local development switcher that routes one stable endpoint
(e.g. `localhost:9000`) to one of many running app servers.

It supports three reverse proxy backends: `native` (pure-Go, default), `traefik`, and `socat`.

## High Level Flow

1. Start proxy (`devswitch proxy start`)
2. Start one or more app servers (`devswitch app start ...`)
3. Update active target (`devswitch switch`)
4. Stop targets or cleanup (`devswitch app stop`, `devswitch cleanup`)

Client traffic always goes through the reverse proxy. Routing changes are applied
immediately — native/socat by updating in-memory state, traefik by rewriting its dynamic config.

## Main Components

### 1) CLI Controller (`internal/devswitch/`)

Responsibilities:

- Choose and manage runtime temp directory
- Track started servers (label + port + PID + branch + command)
- Write provider config files (traefik only)
- Switch active backend
- Start/stop proxy process

Key commands:

- `proxy start [--port PORT] [-b HOST] [--provider PROVIDER]`: start proxy (daemon by default)
- `proxy stop`: stop proxy and all registered app processes
- `info`: show proxy status, port, provider, active backend
- `app start`: launch app on a free port and register it
- `app stop`: stop a selected app
- `list [--json]`: show registered servers
- `switch`: pick active server via interactive selector (`promptui`)
- `cleanup`: stop all registered servers and reset state
- `version`: print build version

### 2) Reverse Proxy Providers (`internal/provider/`)

| Provider  | Implementation                                    | Extra requirement |
| --------- | ------------------------------------------------- | ----------------- |
| `native`  | pure-Go `net/http` reverse proxy with h2c support | none              |
| `traefik` | Traefik process managed by devswitch              | `traefik` binary  |
| `socat`   | TCP-level forwarder                               | `socat` command   |

All providers implement the `ReverseProxy` interface defined in `internal/devswitch/proxy_interface.go`:

```go
type ReverseProxy interface {
    Name() string
    Start(opts provider.StartOptions) (provider.StartResult, error)
    Stop() error
    UpdateRoute(port int, grpc bool) error
    LogPath() string
}
```

### 3) Runtime State Files

All state files are under runtime tmp dir (`DEVSWITCH_TMPDIR` override possible):

| File                    | Purpose                                                |
| ----------------------- | ------------------------------------------------------ |
| `devswitch_servers`     | server registry (label, port, PID, branch, command)    |
| `devswitch_active`      | active target port                                     |
| `proxy.pid`             | daemon proxy PID                                       |
| `proxy.log`             | proxy logs (daemon mode only)                          |
| `proxy.port`            | listen port persisted by `proxy start`                 |
| `proxy.provider`        | current provider name                                  |
| `devswitch_static.yml`  | Traefik static config (traefik provider only)          |
| `devswitch_dynamic.yml` | Traefik dynamic routing config (traefik provider only) |

## Runtime Directory Strategy

- If `DEVSWITCH_TMPDIR` is set, use it directly.
- Otherwise use a state file in `/tmp` keyed by SHA256 of git common dir.
- This key is shared across git worktrees in the same repository.
- `proxy start` always creates a fresh runtime dir for each proxy lifecycle.
- Other commands read the same state to stay in sync.

## Port Selection Priority

For `proxy start`, listen port is resolved in this order:

1. `--port` flag
2. `proxy.port` state file (port used by currently running proxy)
3. `DEVSWITCH_PORT` environment variable
4. Default: `9000`

## App Server Safety

`app start` checks whether the proxy is alive before launching app servers.
If the proxy is not listening, the command returns an error.

## gRPC Routing

When `--grpc` is passed to `app start`:

- native: upstream connection uses h2c transport
- traefik: dynamic service scheme switches from `http` to `h2c`
- Traffic can be tested with `grpcurl -plaintext` via the proxy port

## Data and Control Sequence

1. `proxy start` writes `proxy.port`, `proxy.provider`, starts the proxy process.
2. `app start` starts the app on a free port, stores registry, calls `UpdateRoute`.
3. `switch` calls `UpdateRoute` to point proxy at another registered backend.
4. `proxy stop` terminates the proxy process and removes the runtime directory.
