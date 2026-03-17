package devswitch

import (
	"fmt"
	"os"
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

	// Start UI server in a goroutine (non-blocking daemon mode).
	go func() {
		if err := serveUI(port); err != nil {
			fmt.Printf("UI server error: %v\n", err)
		}
	}()

	return nil
}
