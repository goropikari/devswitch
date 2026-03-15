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

The proxy listens on DEVSWITCH_PORT (default 9000) and forwards traffic
to the currently active app process. The provider can be selected with
--provider or DEVSWITCH_PROXY_PROVIDER:

  native   pure-Go HTTP/1.1 + gRPC reverse proxy (default, no extra binary needed)
  traefik  Traefik-based proxy  (requires traefik binary)
  socat    TCP-level forwarder  (requires socat binary)`,
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
		return nil
	},
}
