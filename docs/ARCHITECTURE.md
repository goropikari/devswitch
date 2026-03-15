# devswitch Architecture

## Purpose

`devswitch` is a local development switcher that routes one stable endpoint
(e.g. `localhost:9000`) to one of many running app servers.

It is built around Traefik and a small controller CLI.

## High Level Flow

1. Start proxy (`devswitch proxy`)
2. Start one or more app servers (`devswitch start-server ...`)
3. Update active target (`devswitch switch`)
4. Stop targets or cleanup (`devswitch stop`, `devswitch cleanup`)

Client traffic always goes through Traefik. Routing changes by rewriting
Traefik dynamic config.

## Main Components

### 1) CLI Controller (`main.go`)

Responsibilities:

- Choose and manage runtime temp directory
- Track started servers (port + PID)
- Write Traefik static/dynamic config
- Switch active backend
- Start/stop proxy process

Key commands:

- `proxy`: start Traefik (daemon by default)
- `proxy-stop`: stop Traefik
- `info`: show proxy log path
- `start-server`: launch app and register it
- `list`: show registered servers
- `switch`: pick active server via interactive selector (`promptui`)
- `stop`: stop selected server
- `cleanup`: stop all registered servers and reset state

### 2) Traefik Proxy

- Static config file points to dynamic config file.
- Dynamic config file contains current active backend target.
- For gRPC mode, backend scheme is `h2c`.

### 3) Runtime State Files

All state files are under runtime tmp dir (`DEVSWITCH_TMPDIR` override possible):

- `devswitch_static.yml`: Traefik static config
- `devswitch_dynamic.yml`: Traefik dynamic routing config
- `devswitch_servers`: server registry (`port pid`)
- `devswitch_active`: active target port
- `proxy.pid`: daemon proxy PID
- `proxy.log`: proxy logs

## Runtime Directory Strategy

- If `DEVSWITCH_TMPDIR` is set, use it directly.
- Otherwise use workspace-based state file in `/tmp`.
- `proxy` command refreshes runtime dir for each proxy lifecycle.
- Other commands read the same state to stay in sync.

## Start-Server Safety

`start-server` checks whether proxy is alive before launching app servers.
If proxy is not listening, command returns an error.

## gRPC Routing

When `--grpc` is passed to `start-server`:

- Dynamic service scheme switches from `http` to `h2c`
- Traffic can be tested with `grpcurl -plaintext` via proxy port

## Data and Control Sequence

1. `proxy` writes config files and starts Traefik.
2. `start-server` starts app on free port, stores registry, updates dynamic config.
3. `switch` rewrites dynamic config to another registered backend.
4. Traefik serves traffic to newly selected backend immediately.
