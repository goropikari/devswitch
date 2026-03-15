package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "show proxy info",
	RunE: func(cmd *cobra.Command, args []string) error {
		proxyImpl, err := currentReverseProxy()
		if err != nil {
			return err
		}

		bindHost := resolveBindHost()
		if bindHost == "" {
			bindHost = "localhost"
		}

		alive := proxyAlive()
		aliveStr := "stopped"
		if alive {
			aliveStr = "running"
		}

		activePort := currentActive()
		activeStr := "-"
		if activePort != 0 {
			activeStr = fmt.Sprintf("%d", activePort)
		}

		fmt.Println("status:        ", aliveStr)
		fmt.Println("provider:      ", proxyImpl.Name())
		fmt.Println("bind:          ", bindHost)
		fmt.Println("port:          ", listenPort())
		fmt.Println("active backend:", activeStr)
		fmt.Println("log:           ", proxyImpl.LogPath())
		fmt.Println("runtime dir:   ", devswitchDir())
		fmt.Println("workspace key: ", workspaceKey())

		if envTmp := os.Getenv("DEVSWITCH_TMPDIR"); envTmp != "" {
			fmt.Println("DEVSWITCH_TMPDIR:", envTmp)
		}
		if envPort := os.Getenv("DEVSWITCH_PORT"); envPort != "" {
			fmt.Println("DEVSWITCH_PORT:  ", envPort)
		}
		if envBind := os.Getenv("DEVSWITCH_BIND_HOST"); envBind != "" {
			fmt.Println("DEVSWITCH_BIND_HOST:", envBind)
		}
		if envProv := os.Getenv("DEVSWITCH_PROXY_PROVIDER"); envProv != "" {
			fmt.Println("DEVSWITCH_PROXY_PROVIDER:", envProv)
		}

		return nil
	},
}
