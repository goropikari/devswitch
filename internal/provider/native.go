package provider

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
)

type nativeProxy struct{ env Env }

func (p nativeProxy) Name() string    { return Native }
func (p nativeProxy) LogPath() string { return p.env.LogFilePath }

// RunServer は純 Go の TCP reverse proxy サーバーを起動してブロックする。
// Env.GetActive() が示すバックエンドポートへ動的に転送する。
func RunServer(env Env) error {
	bindHost := env.BindHost
	if bindHost == "" {
		bindHost = "localhost"
	}
	addr := bindHost + ":" + env.ListenPort
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	defer l.Close()
	log.Printf("TCP proxy listening on %s", addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handleConn(conn, env)
	}
}

func handleConn(clientConn net.Conn, env Env) {
	defer clientConn.Close()

	port := env.GetActive()
	if port == 0 {
		log.Printf("no active backend")
		return
	}
	backendAddr := fmt.Sprintf("127.0.0.1:%d", port)
	backendConn, err := net.Dial("tcp", backendAddr)
	if err != nil {
		log.Printf("failed to connect to backend %s: %v", backendAddr, err)
		return
	}
	defer backendConn.Close()

	// client -> backend
	go func() {
		_, _ = io.Copy(backendConn, clientConn)
		_ = backendConn.Close()
	}()
	// backend -> client
	_, _ = io.Copy(clientConn, backendConn)
	_ = clientConn.Close()
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

func (p nativeProxy) UpdateRoute(port int) error {
	// native proxy は毎リクエストで Env.GetActive() を参照するため
	// SetActive を呼ぶだけでルーティングが即座に切り替わる。
	p.env.SetActive(port)
	return nil
}
