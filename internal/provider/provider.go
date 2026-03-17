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
	Native = "native"
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
	case "", Native:
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
	//nolint:gosec // G304: logPath is internal and safe
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
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

	env.WarnErr("write proxy pid file", os.WriteFile(env.PIDFilePath, []byte(strconv.Itoa(c.Process.Pid)), 0600))

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

// lookupPortPID returns the PID of the process listening on the given TCP port
// by reading /proc/net/tcp{,6} and /proc/<pid>/fd/ without external commands.
func lookupPortPID(port int) int {
	inode := findSocketInode(port)
	if inode == "" {
		return 0
	}
	return findPIDByInode(inode)
}

// findSocketInode looks up the socket inode for a LISTEN entry on port.
func findSocketInode(port int) string {
	hexPort := fmt.Sprintf("%04X", port)
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		//nolint:gosec // G304: Reading /proc/net/tcp{,6} is safe and required for port lookup
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}
			// local_address: IPADDR:PORT (hex); st 0A = TCP_LISTEN
			parts := strings.SplitN(fields[1], ":", 2)
			if len(parts) == 2 && parts[1] == hexPort && fields[3] == "0A" {
				return fields[9] // inode
			}
		}
	}
	return ""
}

// findPIDByInode scans /proc/<pid>/fd symlinks to match the socket inode.
func findPIDByInode(inode string) int {
	target := "socket:[" + inode + "]"
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name()))
			if err != nil {
				continue
			}
			if link == target {
				return pid
			}
		}
	}
	return 0
}

// stopProxy は PID ファイルを使って proxy プロセスを停止する。
func stopProxy(env Env) error {
	port := 0
	if portStr, err := strconv.Atoi(strings.TrimSpace(env.ListenPort)); err == nil {
		port = portStr
	}

	pidData, err := os.ReadFile(env.PIDFilePath)
	if err != nil {
		// PID ファイルがない場合、ポートから PID を lookup して kill する。
		if port > 0 {
			if pid := lookupPortPID(port); pid > 0 {
				if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
					env.WarnErr("send SIGTERM to resolved proxy pid", err)
				}
			}
		}
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

	// ポートがまだ使用中の場合は、port から PID を lookup して kill する（ポート奪取シナリオに対応）。
	if port > 0 {
		if resolvedPID := lookupPortPID(port); resolvedPID > 0 && resolvedPID != pid {
			if err := syscall.Kill(resolvedPID, syscall.SIGKILL); err != nil {
				env.WarnErr("send SIGKILL to conflicting proxy pid", err)
			}
		}
	}

	if err := os.Remove(env.PIDFilePath); err != nil && !os.IsNotExist(err) {
		env.WarnErr("remove proxy pid file", err)
	}

	return nil
}
