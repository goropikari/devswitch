package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var switchCmd = &cobra.Command{
	Use: "switch",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 一覧から選択したサーバーへルーティングを切り替える。
		servers, _ := loadServers()
		s, err := selectServer(servers)
		if err != nil {
			return err
		}

		warnErr("update dynamic config", writeDynamic(s.Port, s.GRPC))
		setActive(s.Port)
		fmt.Println("Switched to:")
		fmt.Printf("Branch: %s\n", formatBranchLabel(s.Branch))
		fmt.Printf("Port: %d\n", s.Port)
		fmt.Printf("PID: %d\n", s.PID)
		if s.Command != "" {
			fmt.Printf("Command: %s\n", s.Command)
		} else {
			fmt.Printf("Command: -\n")
		}
		return nil
	},
}
