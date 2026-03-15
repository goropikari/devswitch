package provider

import (
	"fmt"
	"os/exec"
	"time"
)

type socatProxy struct{ env Env }

func (p socatProxy) Name() string    { return Socat }
func (p socatProxy) LogPath() string { return p.env.LogFilePath }

func (p socatProxy) Start(opts StartOptions) (StartResult, error) {
	if err := p.env.EnsureTmpDir(); err != nil {
		return StartResult{}, err
	}

	target := p.env.GetActive()
	if target == 0 {
		target = 65535
	}

	return p.startToTarget(target, opts)
}

func (p socatProxy) Stop() error {
	return stopProxy(p.env)
}

func (p socatProxy) UpdateRoute(port int, grpc bool) error {
	_ = grpc // socat は L4 転送のため h2c/http の区別は不要

	// 起動中の socat を止めて、選択ポート向けに再起動する。
	if err := stopProxy(p.env); err != nil {
		return err
	}

	// fork した子プロセスも含めてポートが解放されるまで待つ。
	if err := waitPortFree(p.env.ListenPort, 2*time.Second); err != nil {
		return fmt.Errorf("listen port %s not released: %w", p.env.ListenPort, err)
	}

	_, err := p.startToTarget(port, StartOptions{Daemon: true})
	return err
}

func (p socatProxy) startToTarget(target int, opts StartOptions) (StartResult, error) {
	bindHost := p.env.BindHost
	if bindHost == "" {
		bindHost = "localhost"
	}
	c := exec.Command(
		"socat",
		fmt.Sprintf("TCP-LISTEN:%s,bind=%s,reuseaddr,fork", p.env.ListenPort, bindHost),
		fmt.Sprintf("TCP:127.0.0.1:%d", target),
	)

	if opts.Daemon {
		return startProcessDaemon(c, p.env)
	}

	return StartResult{}, startProcessForeground(c)
}
