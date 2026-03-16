package provider

// proxyContract は provider 実装が満たすべき静的契約。
// 実行時には使わず、コンパイル時の検証だけに使う。
type proxyContract interface {
	Name() string
	Start(opts StartOptions) (StartResult, error)
	Stop() error
	UpdateRoute(port int, grpc bool) error
	LogPath() string
}

var _ proxyContract = (*nativeProxy)(nil)
