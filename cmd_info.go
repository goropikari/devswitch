package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "show proxy info",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("port:", listenPort())
		fmt.Println("log:", proxyLogFilePath())
		return nil
	},
}
