# devswitch アーキテクチャ

## 目的

`devswitch` は、1つの固定エンドポイント
（例: `localhost:9000`）を、起動中の複数アプリサーバーのうち
任意の1台へ切り替えてルーティングするローカル開発用スイッチャーです。

Traefik と小さな制御用 CLI を中心に構成されています。

## 全体フロー

1. proxy を起動（`devswitch proxy`）
2. 1つ以上のアプリサーバーを起動（`devswitch start-server ...`）
3. 接続先を切り替え（`devswitch switch`）
4. 停止またはクリーンアップ（`devswitch stop`, `devswitch cleanup`）

クライアントの通信は常に Traefik を経由します。
ルーティングは Traefik の dynamic config を書き換えることで変更します。

## 主要コンポーネント

### 1) CLI コントローラー（`main.go`）

責務:

- 実行時 tmp ディレクトリの決定と管理
- 起動済みサーバー（port + PID）の追跡
- Traefik の static/dynamic 設定ファイル生成
- アクティブ backend の切り替え
- proxy プロセスの起動/停止

主要コマンド:

- `proxy`: Traefik を起動（デフォルトは daemon）
- `proxy-stop`: Traefik を停止
- `info`: proxy の port とログパスを表示
- `start-server`: アプリを起動して登録
- `list`: 登録済みサーバーを表示
- `switch`: 対話セレクタ（`promptui`）でアクティブサーバーを選択
- `stop`: 選択したサーバーを停止
- `cleanup`: 登録済みサーバーを全停止し状態を初期化

### 2) Traefik Proxy

- static config は dynamic config を参照します。
- dynamic config は現在のアクティブ backend を保持します。
- gRPC モードでは backend scheme に `h2c` を使います。

### 3) 実行時状態ファイル

すべての状態ファイルは実行時 tmp ディレクトリ配下に作成されます
（`DEVSWITCH_TMPDIR` で上書き可能）。

- `devswitch_static.yml`: Traefik static config
- `devswitch_dynamic.yml`: Traefik dynamic routing config
- `devswitch_servers`: サーバーレジストリ（`port pid`）
- `devswitch_active`: アクティブ対象 port
- `proxy.pid`: daemon proxy の PID
- `proxy.log`: proxy ログ

## 実行ディレクトリ戦略

- `DEVSWITCH_TMPDIR` が設定されている場合はその値を使用
- 未設定の場合は `/tmp` 配下の state file を使って実行時ディレクトリを決定
- `proxy` コマンド実行時に、proxy ライフサイクル用の実行時ディレクトリを確定
- ほかのコマンドは同じ state を読み取り、同一ディレクトリを共有

## Start-Server の安全性

`start-server` はアプリ起動前に proxy の生存確認を行います。
proxy が listen していない場合、コマンドはエラーを返して終了します。

## gRPC ルーティング

`start-server` に `--grpc` を付けた場合:

- dynamic service の scheme を `http` から `h2c` へ変更
- proxy ポート経由で `grpcurl -plaintext` による確認が可能

## データ/制御シーケンス

1. `proxy` が設定ファイルを書き込み、Traefik を起動
2. `start-server` が空き port でアプリを起動し、レジストリと dynamic config を更新
3. `switch` が dynamic config を別 backend に書き換え
4. Traefik が新しい backend へ即時にルーティング
