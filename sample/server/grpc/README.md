# gRPC サンプル README

## ディレクトリ構成

```
grpc/
├── go.mod
├── go.sum
├── main.go
└── proto/
    ├── hello.proto
    ├── hello.pb.go
    └── hello_grpc.pb.go
```

## proto ファイルから Go コード生成

`protoc` コマンドを grpc ディレクトリで実行します。

```sh
protoc --go_out=proto --go-grpc_out=proto --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative ./hello.proto
```

- `--go_out=proto` : Goコードを proto ディレクトリに出力
- `--go-grpc_out=proto` : gRPCコードを proto ディレクトリに出力
- `--go_opt=paths=source_relative` : 相対パスで出力

## gRPC サーバーの起動

```
go run main.go 50051
```

## grpcurl でリクエスト

サーバー起動後、以下のコマンドでリクエストできます。

```sh
grpcurl -plaintext localhost:50051 hello.HelloService/SayHello
```

- `-plaintext` : TLSなし
- `localhost:50051` : サーバーアドレス
- `hello.HelloService/SayHello` : サービス/メソッド

## 補足
- `protoc` や `grpcurl` のインストールが必要です。
- Goモジュールは grpc ディレクトリ内で管理しています。
