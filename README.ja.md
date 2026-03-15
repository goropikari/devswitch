# devswitch (日本語)

英語版 README は [README.md](README.md) を参照してください。

`devswitch` は **ローカルで複数の開発サーバーを起動し、1つの固定 URL から接続先を切り替えるための CLI ツール**です。

リバースプロキシ（native/Traefik/socat）と組み合わせ、
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
reverse proxy (native / Traefik / socat)
  │
  ▼
active dev server
```

`devswitch` の役割

* 開発サーバーを **空いているポートで起動**
* 起動したサーバーを **label・branch・port・PID とともに registry に記録**
* **proxy のルーティングを更新**
* 接続先サーバーを **対話セレクタ（promptui）で切り替え**
* `git worktree` 間で状態を共有

---

# Requirements

必要なツール

* Go 1.26+
* 対話可能なターミナル（TTY）

プロバイダーごとの追加要件

| プロバイダー | 追加要件 |
| ------------ | -------- |
| `native`（デフォルト） | なし（純 Go 実装） |
| `traefik` | [Traefik バイナリ](https://doc.traefik.io/traefik/getting-started/install-traefik/) |
| `socat` | socat コマンド |

---

# Installation

```bash
git clone https://github.com/goropikari/devswitch.git
cd devswitch
go build -o devswitch ./cmd/devswitch
```

## go install で入れる

```bash
go install github.com/goropikari/devswitch/cmd/devswitch@latest
```

---

# Environment Variables

| 変数 | 説明 | default | 影響するコマンド |
| ---- | ---- | ------- | ---------------- |
| `DEVSWITCH_PORT` | proxy listen port | `9000` | proxy start, app start, info |
| `DEVSWITCH_BIND_HOST` | proxy bind host | `localhost` | proxy start, info |
| `DEVSWITCH_PROXY_PROVIDER` | proxy プロバイダー (`native`\|`traefik`\|`socat`) | `native` | proxy start, info |
| `DEVSWITCH_TMPDIR` | 状態・ログ・設定ファイルの保存先 | 自動生成 (`/tmp/...`) | 全コマンド |

---

# Usage

## 1. Proxy を起動

```bash
devswitch proxy start
```

デフォルトでは daemon で起動します。

```bash
# listen port を変更する（デフォルト: 9000）
# --port フラグは DEVSWITCH_PORT 環境変数より優先される
devswitch proxy start --port 8080
# または
DEVSWITCH_PORT=8080 devswitch proxy start

# バインドホストを指定（devcontainer → ホストからアクセスする場合）
devswitch proxy start -b 0.0.0.0

# プロバイダーを指定
devswitch proxy start --provider traefik

# 停止
devswitch proxy stop

# 状態確認
devswitch info
```

アクセス：

```
http://localhost:9000
```

---

## 2. アプリを起動

`devswitch app start` は空きポートを自動で割り当て、アプリに渡します。

### 環境変数でポートを渡す

```bash
devswitch app start --port-env PORT -- python -m http.server
# => PORT=54321 python -m http.server
```

### フラグでポートを渡す

```bash
devswitch app start --port-arg --port -- go run ./sample/server/http/main.go
# => go run ./sample/server/http/main.go --port 54321
```

### ラベルを指定する

同じブランチで複数のサーバーを起動したとき、選択しやすくするためにラベルを付けられます。
省略するとランダムな名前（Docker 風の `adjective_noun`）が自動生成されます。

```bash
devswitch app start --label my-feature --port-env PORT -- ./myapp
devswitch app start -l debug-build --port-arg --port -- ./myapp
```

### gRPC サーバー

```bash
devswitch app start --grpc --port-arg --port -- go -C ./sample/server/grpc run main.go
# => go -C ./sample/server/grpc run main.go --port 54321  (h2c ルーティング)
```

#### grpcurl サンプル

```bash
grpcurl -plaintext localhost:9000 list
grpcurl -plaintext localhost:9000 list hello.HelloService
grpcurl -plaintext -d '{}' localhost:9000 hello.HelloService/SayHello
```

---

## 3. 接続先を切り替える

```bash
devswitch switch
```

対話セレクタが開き、ラベル・ブランチ・ポートで選択できます。

```
  happy_turing           branch=[main]          port=54321 pid=12345 cmd=...
  nervous_hopper         branch=[feature/login]  port=54322 pid=12346 cmd=...
```

---

## 4. 一覧表示

```bash
devswitch list
```

```
LABEL                  BRANCH            PORT     PID      ACTIVE CMD
happy_turing           [main]            54321    12345    *      PORT=54321 ./myapp
nervous_hopper         [feature/login]   54322    12346           PORT=54322 ./myapp
```

アクティブなプロセスには `*` が付きます。

---

## 5. アプリを停止

```bash
devswitch app stop
```

アクティブなアプリを停止した場合、残りのアプリへ自動的に切り替わります。

---

## 6. 全停止・初期化

```bash
devswitch cleanup
```

登録済みの全アプリを停止し、registry と active 状態をリセットします。

---

# Devcontainer Usage

devcontainer では **1つの port だけ forward**すれば十分です。

```json
"forwardPorts": [9000]
```

ホスト側からアクセスできるようにするには bind host を `0.0.0.0` に設定します。

```bash
DEVSWITCH_BIND_HOST=0.0.0.0 devswitch proxy start
# または
devswitch proxy start -b 0.0.0.0
```

---

# Runtime Files

| path | 用途 |
| ---- | ---- |
| `<tmpdir>/devswitch_static.yml` | Traefik static config |
| `<tmpdir>/devswitch_dynamic.yml` | routing config |
| `<tmpdir>/devswitch_servers` | server registry |
| `<tmpdir>/devswitch_active` | アクティブ対象 port |
| `<tmpdir>/proxy.pid` | proxy daemon PID |
| `<tmpdir>/proxy.log` | proxy ログ（daemon 時のみ） |
| `<tmpdir>/proxy.provider` | 使用中のプロバイダー名 |

---

# License

MIT
