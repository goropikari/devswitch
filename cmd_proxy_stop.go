package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop proxy daemon",
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
