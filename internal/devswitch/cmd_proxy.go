package devswitch

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/goropikari/devswitch/internal/provider"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "manage the reverse proxy",
	Long:  `Start, stop, and manage the reverse proxy that forwards incoming traffic to app processes.`,
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "start the reverse proxy daemon",
	Long: `Start the reverse proxy in daemon mode (default) or foreground.

Listen port priority (highest to lowest):
  1. --port flag
  2. DEVSWITCH_PORT environment variable
  3. default (9000)

The provider can be selected with --provider or DEVSWITCH_PROXY_PROVIDER:

  native   pure-Go HTTP/1.1 + gRPC reverse proxy (default, no extra binary needed)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if proxyPort != "" {
			if err := os.Setenv("DEVSWITCH_PORT", proxyPort); err != nil {
				logJSON("set DEVSWITCH_PORT", fmt.Sprintf("port=%s", proxyPort), err)
				return err
			}
		}
		// proxy 起動ごとに tmp dir を新規確定する。
		if os.Getenv("DEVSWITCH_TMPDIR") == "" {
			if _, err := initRuntimeDir(true); err != nil {
				return err
			}
		}
		// 使用ポートを state ファイルに保存（他コマンドが参照できるよう）。
		if err := os.WriteFile(proxyPortFilePath(), []byte(listenPort()), 0600); err != nil {
			logJSON("write proxy port file", fmt.Sprintf("port=%s", listenPort()), err)
		}

		providerName := strings.TrimSpace(proxyProvider)
		if providerName == "" {
			providerName = resolveCurrentProxyProviderName()
		}

		proxyImpl, err := newReverseProxy(providerName)
		if err != nil {
			return err
		}

		res, err := proxyImpl.Start(provider.StartOptions{Daemon: proxyDaemon})
		if err != nil {
			return err
		}

		if err := writeProxyProviderName(proxyImpl.Name()); err != nil {
			logJSON("write proxy provider name", fmt.Sprintf("provider=%s", proxyImpl.Name()), err)
		}

		if proxyDaemon {
			fmt.Println("proxy started in daemon mode", res.PID)
			host := resolveBindHost()
			if host == "" {
				host = "localhost"
			}
			fmt.Printf("Proxy Listen: %s:%s\n", host, listenPort())
			fmt.Println("proxy provider", proxyImpl.Name())
			fmt.Println("proxy log", res.LogPath)
		}

		// Start UI server in daemon mode
		if uiDaemon {
			if err := startUIDaemon(); err != nil {
				return err
			}
		}

		return nil
	},
}

func startUIDaemon() error {
	port := uiPort
	if port == "" {
		port = os.Getenv("DEVSWITCH_UI_PORT")
	}
	if port == "" {
		port = "9001"
	}

	bindHost := uiBindHost
	if bindHost == "" {
		bindHost = os.Getenv("DEVSWITCH_UI_BIND_HOST")
	}
	if bindHost == "" {
		bindHost = "localhost"
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	//nolint:gosec // G204: Trusting os.Executable() to launch UI daemon
	cmd := exec.Command(exe, "__ui-serve", "--port", port, "--bind", bindHost)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	logPath := uiLogFilePath()
	//nolint:gosec // G304: logPath is internal and safe
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open UI log file: %w", err)
	}
	// We do not close logFile here because it is attached to the child process.
	// The file descriptor will be closed in the parent process when this function returns/cmd.Start() finishes?
	// Actually, exec.Command duplicates the file descriptor. We should close it in parent after Start().
	defer logFile.Close()

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start UI daemon: %w", err)
	}

	if err := os.WriteFile(uiPIDFilePath(), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
		logJSON("write UI pid file", fmt.Sprintf("pid=%d", cmd.Process.Pid), err)
	}
	if err := os.WriteFile(uiPortFilePath(), []byte(port), 0600); err != nil {
		logJSON("write UI port file", fmt.Sprintf("port=%s", port), err)
	}

	fmt.Println("UI started in daemon mode", cmd.Process.Pid)
	fmt.Printf("UI URL: http://%s:%s\n", bindHost, port)
	return nil
}
