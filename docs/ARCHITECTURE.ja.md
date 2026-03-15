# devswitch アーキテクチャ

## 目的

`devswitch` は、ローカル開発環境で 1 つの固定エンドポイント
（例: `localhost:9000`）を、起動中の複数アプリサーバーのうち
任意の 1 台へ切り替えてルーティングするローカル開発用スイッチャーです。

3 種類のリバースプロキシバックエンドをサポートします: `native`（純 Go、デフォルト）、`traefik`、`socat`。

## 全体フロー

1. proxy を起動（`devswitch proxy start`）
2. アプリサーバーを起動（`devswitch app start ...`）
3. 接続先を切り替え（`devswitch switch`）
4. 停止・初期化（`devswitch app stop`、`devswitch cleanup`）

クライアントの通信は常にリバースプロキシを経由します。
native/socat はアクティブポートをインメモリで管理し、traefik は dynamic config を書き換えることで即座にルーティング変更を適用します。

## 主要コンポーネント

### 1) CLI コントローラー（`internal/devswitch/`）

責務:

- 実行時 tmp ディレクトリの決定と管理
- 起動済みサーバー（label + port + PID + branch + command）の追跡
- プロバイダー設定ファイルの生成（traefik のみ）
- アクティブ backend の切り替え
- proxy プロセスの起動/停止

主要コマンド:

- `proxy start [--port PORT] [-b HOST] [--provider PROVIDER]`: proxy を起動（デフォルトは daemon）
- `proxy stop`: proxy と登録済みアプリを全停止
- `info`: proxy のステータス・ポート・プロバイダー・アクティブ backend を表示
- `app start`: 空きポートでアプリを起動し登録
- `app stop`: 選択したアプリを停止
- `list [--json]`: 登録済みサーバーを表示
- `switch`: 対話セレクタ（`promptui`）でアクティブサーバーを選択
- `cleanup`: 登録済みサーバーを全停止し状態を初期化
- `version`: ビルドバージョンを表示

### 2) リバースプロキシプロバイダー（`internal/provider/`）

| プロバイダー | 実装                                          | 追加要件           |
| ------------ | --------------------------------------------- | ------------------ |
| `native`     | 純 Go `net/http` リバースプロキシ（h2c 対応） | なし               |
| `traefik`    | devswitch が管理する Traefik プロセス         | `traefik` バイナリ |
| `socat`      | TCP レベルフォワーダー                        | `socat` コマンド   |

全プロバイダーは `internal/devswitch/proxy_interface.go` で定義する `ReverseProxy` インターフェースを実装します:

```go
type ReverseProxy interface {
    Name() string
    Start(opts provider.StartOptions) (provider.StartResult, error)
    Stop() error
    UpdateRoute(port int, grpc bool) error
    LogPath() string
}
```

### 3) 実行時状態ファイル

全ファイルは実行時 tmp ディレクトリ配下に作成されます（`DEVSWITCH_TMPDIR` で上書き可能）:

| ファイル                | 用途                                                    |
| ----------------------- | ------------------------------------------------------- |
| `devswitch_servers`     | サーバーレジストリ（label・port・PID・branch・command） |
| `devswitch_active`      | アクティブ対象 port                                     |
| `proxy.pid`             | daemon proxy の PID                                     |
| `proxy.log`             | proxy ログ（daemon 時のみ）                             |
| `proxy.port`            | `proxy start` で確定した listen port                    |
| `proxy.provider`        | 使用中のプロバイダー名                                  |
| `devswitch_static.yml`  | Traefik static config（traefik のみ）                   |
| `devswitch_dynamic.yml` | Traefik dynamic routing config（traefik のみ）          |

## 実行ディレクトリ戦略

- `DEVSWITCH_TMPDIR` が設定されている場合はその値を使用
- 未設定の場合は `/tmp` 配下の state file を使って実行時ディレクトリを決定
- state file のキーは git common dir の SHA256 で、同一リポジトリの worktree 間で共有
- `proxy start` 実行時に、proxy ライフサイクル用の実行時ディレクトリを常に新規作成
- その他のコマンドは同じ state を読み取り、同一ディレクトリを参照

## ポート決定優先順位

`proxy start` の listen port は次の順で決定されます:

1. `--port` フラグ
2. `proxy.port` state ファイル（現在起動中の proxy のポート）
3. `DEVSWITCH_PORT` 環境変数
4. デフォルト: `9000`

## アプリ起動時の安全性

`app start` はアプリ起動前に proxy の生存確認を行います。
proxy が listen していない場合、コマンドはエラーを返して終了します。

## gRPC ルーティング

`app start` に `--grpc` を付けた場合:

- native: アップストリーム接続に h2c トランスポートを使用
- traefik: dynamic service の scheme を `http` から `h2c` へ変更
- proxy ポート経由で `grpcurl -plaintext` によるテストが可能

## データ/制御シーケンス

1. `proxy start` が `proxy.port`、`proxy.provider` を書き込み、proxy プロセスを起動
2. `app start` が空き port でアプリを起動し、レジストリを更新し、`UpdateRoute` を呼び出す
3. `switch` が `UpdateRoute` を呼び出して別の backend へルーティングを切り替え
4. `proxy stop` が proxy プロセスを終了させ、実行時ディレクトリを削除
