package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/goropikari/devswitch/provider"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "manage proxy",
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "start proxy daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		// proxy 起動ごとに tmp dir を新規確定する。
		if os.Getenv("DEVSWITCH_TMPDIR") == "" {
			if _, err := initRuntimeDir(true); err != nil {
				return err
			}
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

		warnErr("write proxy provider", writeProxyProviderName(proxyImpl.Name()))

		if proxyDaemon {
			fmt.Println("proxy started in daemon mode", res.PID)
			fmt.Println("proxy provider", proxyImpl.Name())
			fmt.Println("proxy log", res.LogPath)
		}
		return nil
	},
}
