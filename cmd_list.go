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

		fmt.Printf("%-17s %-8s %-8s %-6s %s\n", "BRANCH", "PORT", "PID", "ACTIVE", "CMD")
		for _, s := range servers {
			branch := formatBranchLabel(s.Branch)
			runCmd := s.Command
			if runCmd == "" {
				runCmd = "-"
			}
			mark := ""
			if s.Port == active {
				mark = "*"
			}
			fmt.Printf("%-17s %-8d %-8d %-6s %s\n", branch, s.Port, s.PID, mark, runCmd)
		}
		return nil
	},
}
