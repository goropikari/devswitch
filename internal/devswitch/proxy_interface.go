package devswitch

import "github.com/goropikari/devswitch/internal/provider"

// ReverseProxy は devswitch 側で扱う reverse proxy の抽象化。
type ReverseProxy interface {
	Name() string
	Start(opts provider.StartOptions) (provider.StartResult, error)
	Stop() error
	UpdateRoute(port int) error
	LogPath() string
}
