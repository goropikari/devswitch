# devswitch (日本語)

英語版 README は [README.md](README.md) を参照してください。

`devswitch` は **ローカルで複数の開発サーバーを起動し、1つの固定 URL から接続先を切り替えるための CLI ツール**です。

内部ではリバースプロキシとして **Traefik** を利用し、`dynamic.yml` を書き換えることで
**HTTP / gRPC サーバーの接続先を即座に切り替え**ます。

主な用途：

* microservice の並列開発
* AI agent による複数 server 起動
* devcontainer での single port forwarding
* `git worktree` を使った複数ブランチ開発

---

# Architecture

```
client
  │
  ▼
localhost:9000
  │
  ▼
Traefik (reverse proxy)
  │
  ▼
active dev server
```

`devswitch` の役割

* 開発サーバーを **空いているポートで起動**
* 起動したサーバーを **registry に記録**
* **Traefik の dynamic config を更新**
* 接続先サーバーを **対話セレクタ（promptui）で切り替え**

---

# Requirements

必要なツール

* Go 1.26+
* **Traefik**
* 対話可能なターミナル（TTY）

インストール例

追加の選択コマンドは不要です。`devswitch` が内部で `promptui` を利用します。

Traefik は公式ドキュメントの Linux 手順で導入してください。
<https://doc.traefik.io/traefik/getting-started/install-traefik/>

---

# Installation

```bash
git clone https://github.com/goropikari/devswitch.git
cd devswitch

go build -o devswitch
```

## go install で入れる

以下で `GOBIN`（または `GOPATH/bin`）にインストールできます。

```bash
go install github.com/goropikari/devswitch@latest
```

インストール後は次のように実行できます。

```bash
devswitch proxy start
```

---

# Environment Variables

| 変数             | 説明                                              | default            |
| ---------------- | ------------------------------------------------- | ------------------ |
| `DEVSWITCH_PORT` | proxy listen port                                 | `9000`             |
| `DEVSWITCH_TMPDIR` | state / log / traefik config を保存するディレクトリ | 自動生成 (`/tmp/...`) |

例

```bash
export DEVSWITCH_PORT=9000
export DEVSWITCH_TMPDIR=/tmp/devswitch
```

---

# Usage

## 1. Proxy を起動

```bash
devswitch proxy start
```

デフォルトでは daemon で起動します。停止には `devswitch proxy stop` を使います。

ログファイルの場所は次で確認できます。

```bash
devswitch info
```

これにより **Traefik** が起動し、以下でアクセス可能になります。

```
http://localhost:9000
```

---

# 2. 開発サーバーを起動

## 環境変数で port を渡す

サーバーが `PORT` を読む場合

```bash
devswitch start-server \
  --port-env PORT \
  <your-command>
```

実際には

```
PORT=42131 <your-command>
```

のように実行されます。

---

## フラグで port を渡す

サーバーが `--port` を使う場合

```bash
devswitch start-server \
  --port-arg --port \
  -- go run ./http/main.go
```

実行されるコマンド

```
go run ./http/main.go --port 42131
```

---

# gRPC server

gRPC サーバーは `--grpc` を指定します。

```bash
devswitch start-server \
  --grpc \
  --port-arg --port \
  -- go -C ./grpc run main.go
```

Traefik では

```
h2c://localhost:<port>
```

として接続されます。

## grpcurl サンプル

proxy (`devswitch proxy start`) と gRPC server 起動後、
default port (`9000`) に対して grpcurl を実行できます。

### 利用可能な service 一覧

```bash
grpcurl -plaintext localhost:9000 list
```

### 利用可能な method 一覧

```bash
grpcurl -plaintext localhost:9000 list hello.HelloService
```

### SayHello を呼び出す

```bash
grpcurl -plaintext -d '{}' localhost:9000 hello.HelloService/SayHello
```

レスポンス例

```json
{
  "message": "Hello from port 42131"
}
```

---

# Server Switching

複数の server を起動したあと

```bash
devswitch switch
```

すると対話セレクタが開き、接続先 server を選択できます。

```
branch=[main] port=42131 pid=12345
branch=[feature/login] port=42140 pid=12360
branch=[fix/api] port=42155 pid=12388
```

選択すると **Traefik routing が即座に更新**されます。

`devswitch list` は `BRANCH PORT PID ACTIVE` の順で表示されます。

---

# Devcontainer Usage

devcontainer では **1つの port だけ forward**すれば十分です。

```json
"forwardPorts": [9000]
```

すべての開発サーバーに

```
http://localhost:9000
```

でアクセスできます。

---

# Registry

起動した server は

```
<tmpdir>/devswitch_servers
```

に記録されます。

フォーマット

```
port pid
```

---

# Files

| path                            | 用途                  |
| ------------------------------- | --------------------- |
| `<tmpdir>/devswitch_static.yml` | Traefik static config |
| `<tmpdir>/devswitch_dynamic.yml`| routing config        |
| `<tmpdir>/devswitch_servers`    | server registry       |
| `<tmpdir>/proxy.log`            | proxy log             |

---

# License

MIT
