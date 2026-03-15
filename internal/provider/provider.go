package provider

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	Traefik = "traefik"
	Socat   = "socat"
	Native  = "native"
)

type StartOptions struct {
	Daemon bool
}

type StartResult struct {
	PID     int
	LogPath string
}

// Env はプロバイダーが必要とする状態・関数を保持する。
type Env struct {
	BindHost         string // バインドするホスト (例: "0.0.0.0", "127.0.0.1")
	ListenPort       string
	LogFilePath      string
	PIDFilePath      string
	DynConfigPath    string
	StaticConfigPath string
	Executable       string
	GetActive        func() int
	SetActive        func(int)
	EnsureTmpDir     func() error
	PIDAlive         func(int) bool
	WarnErr          func(action string, err error)
}

func New(name string, env Env) (any, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case Traefik:
		return traefikProxy{env: env}, nil
	case Socat:
		return socatProxy{env: env}, nil
	case Native:
		return nativeProxy{env: env}, nil
	default:
		return nil, fmt.Errorf("unsupported proxy provider: %s", name)
	}
}

// waitPortFree は指定ポートが timeout 以内に解放されるまでポーリングする。
func waitPortFree(port string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 100*time.Millisecond)
		if err != nil {
			// 接続できなければポートが解放済み
			return nil
		}
		_ = conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %s to be free", port)
}

func startProcessDaemon(c *exec.Cmd, env Env) (StartResult, error) {
	logPath := env.LogFilePath
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return StartResult{}, err
	}

	c.Stdout = logFile
	c.Stderr = logFile
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := c.Start(); err != nil {
		env.WarnErr("close proxy log file", logFile.Close())
		return StartResult{}, err
	}

	env.WarnErr("write proxy pid file", os.WriteFile(env.PIDFilePath, []byte(strconv.Itoa(c.Process.Pid)), 0644))

	// 起動直後に死んでいないか確認して、誤検知の「started」を防ぐ。
	time.Sleep(200 * time.Millisecond)
	if !env.PIDAlive(c.Process.Pid) {
		if err := os.Remove(env.PIDFilePath); err != nil && !os.IsNotExist(err) {
			env.WarnErr("remove proxy pid file", err)
		}
		env.WarnErr("close proxy log file", logFile.Close())
		return StartResult{}, fmt.Errorf("proxy failed to start; see log: %s", logPath)
	}

	env.WarnErr("close proxy log file", logFile.Close())
	return StartResult{PID: c.Process.Pid, LogPath: logPath}, nil
}

func startProcessForeground(c *exec.Cmd) error {
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// stopProxy は PID ファイルを使って proxy プロセスを停止する。
func stopProxy(env Env) error {
	portTarget := fmt.Sprintf("%s/tcp", env.ListenPort)

	pidData, err := os.ReadFile(env.PIDFilePath)
	if err != nil {
		// PID ファイルがなくても待受ポートを閉じに行く。
		_ = exec.Command("fuser", "-k", portTarget).Run()
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		return fmt.Errorf("invalid proxy pid: %w", err)
	}

	if env.PIDAlive(pid) {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return err
		}
	}

	// fuser -k は対象プロセスが存在しない場合も exit status 1 を返すため無視する。
	_ = exec.Command("fuser", "-k", portTarget).Run()

	if err := os.Remove(env.PIDFilePath); err != nil && !os.IsNotExist(err) {
		env.WarnErr("remove proxy pid file", err)
	}

	return nil
}
