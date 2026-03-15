package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list running app processes",
	Long: `Show all app processes registered in the current workspace.
The active backend (currently receiving proxy traffic) is marked with *.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 登録済みサーバーと現在の active を一覧表示する。
		servers, _ := loadServers()
		active := currentActive()

		fmt.Printf("%-22s %-17s %-8s %-8s %-6s %s\n", "LABEL", "BRANCH", "PORT", "PID", "ACTIVE", "CMD")
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
			fmt.Printf("%-22s %-17s %-8d %-8d %-6s %s\n", s.Label, branch, s.Port, s.PID, mark, runCmd)
		}
		return nil
	},
}
