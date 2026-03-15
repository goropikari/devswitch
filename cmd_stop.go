package main

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use: "stop",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 一覧から選んだサーバーのポートを停止する。
		servers, _ := loadServers()
		s, err := selectServer(servers)
		if err != nil {
			return err
		}

		if err := exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", s.Port)).Run(); err != nil {
			warnErr(fmt.Sprintf("kill tcp port=%d", s.Port), err)
		}
		fmt.Println("stopped", s.Port)
		return nil
	},
}
