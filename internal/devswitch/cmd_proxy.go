package devswitch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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
			os.Setenv("DEVSWITCH_PORT", proxyPort)
		}
		// proxy 起動ごとに tmp dir を新規確定する。
		if os.Getenv("DEVSWITCH_TMPDIR") == "" {
			if _, err := initRuntimeDir(true); err != nil {
				return err
			}
		}
		// 使用ポートを state ファイルに保存（他コマンドが参照できるよう）。
		warnErr("write proxy port", os.WriteFile(proxyPortFilePath(), []byte(listenPort()), 0644))

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

		warnErr("write proxy provider", writeProxyProviderName(proxyImpl.Name()))

		if proxyDaemon {
			fmt.Println("proxy started in daemon mode", res.PID)
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

	exe, _ := os.Executable()
	c := exec.Command(exe, "__ui-serve", "--port", port)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	// c.Stdin = os.Stdin // Daemon usually doesn't need stdin.

	if err := c.Start(); err != nil {
		return err
	}

	// We don't print "UI started in daemon mode" here because __ui-serve prints "UI started at ..."
	// But __ui-serve output goes to Stdout which might be piped or file.
	// If proxy is daemon, its stdout is logged to file.
	// So this print is fine.
	fmt.Println("UI started in daemon mode", c.Process.Pid)
	return nil
}
