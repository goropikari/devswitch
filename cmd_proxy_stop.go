package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop the reverse proxy daemon",
	Long:  `Stop the running reverse proxy and terminate all registered app processes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		proxyImpl, err := currentReverseProxy()
		if err != nil {
			return err
		}

		if err := proxyImpl.Stop(); err != nil {
			return err
		}
		fmt.Println("proxy stop requested", listenPort())
		return cleanupAllStartedServers()
	},
}
