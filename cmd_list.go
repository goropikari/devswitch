package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use: "list",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 登録済みサーバーと現在の active を一覧表示する。
		servers, _ := loadServers()
		active := currentActive()

		fmt.Printf("%-8s %-8s %-6s\n", "PORT", "PID", "ACTIVE")
		for _, s := range servers {
			mark := ""
			if s.Port == active {
				mark = "*"
			}
			fmt.Printf("%-8d %-8d %-6s\n", s.Port, s.PID, mark)
		}
		return nil
	},
}
