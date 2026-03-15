package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type nativeProxy struct{ env Env }

func (p nativeProxy) Name() string    { return Native }
func (p nativeProxy) LogPath() string { return p.env.LogFilePath }

// smartTransport は Content-Type: application/grpc のリクエストには h2c トランスポートを使い、
// それ以外は通常の HTTP/1.1 トランスポートを使う。
type smartTransport struct {
	h1 *http.Transport
	h2 *http2.Transport
}

func newSmartTransport() *smartTransport {
	return &smartTransport{
		h1: &http.Transport{},
		h2: &http2.Transport{
			AllowHTTP: true,
			// TLS なしで h2c バックエンドへ接続する。
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, addr)
			},
		},
	}
}

func (t *smartTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasPrefix(req.Header.Get("Content-Type"), "application/grpc") {
		return t.h2.RoundTrip(req)
	}
	return t.h1.RoundTrip(req)
}

// RunServer は純 Go の reverse proxy サーバーを起動してブロックする。
// HTTP/1.1 と h2c (gRPC) の両方を受け付け、Env.GetActive() が示す
// バックエンドポートへ動的に転送する。
func RunServer(env Env) error {
	transport := newSmartTransport()

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			port := env.GetActive()
			req.URL.Scheme = "http"
			req.URL.Host = fmt.Sprintf("127.0.0.1:%d", port)
			req.Host = req.URL.Host
		},
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "bad gateway: "+err.Error(), http.StatusBadGateway)
		},
	}

	h2s := &http2.Server{}
	bindHost := env.BindHost
	if bindHost == "" {
		bindHost = "localhost"
	}
	return http.ListenAndServe(bindHost+":"+env.ListenPort, h2c.NewHandler(rp, h2s))
}

func (p nativeProxy) Start(opts StartOptions) (StartResult, error) {
	if err := p.env.EnsureTmpDir(); err != nil {
		return StartResult{}, err
	}

	exe := p.env.Executable
	if exe == "" {
		var err error
		exe, err = os.Executable()
		if err != nil {
			return StartResult{}, fmt.Errorf("resolve executable: %w", err)
		}
	}

	c := exec.Command(exe, "__proxy-serve")
	if opts.Daemon {
		return startProcessDaemon(c, p.env)
	}

	return StartResult{}, startProcessForeground(c)
}

func (p nativeProxy) Stop() error {
	return stopProxy(p.env)
}

func (p nativeProxy) UpdateRoute(port int, grpc bool) error {
	// native proxy は毎リクエストで Env.GetActive() を参照するため
	// SetActive を呼ぶだけでルーティングが即座に切り替わる。
	p.env.SetActive(port)
	return nil
}
