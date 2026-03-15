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

		warnErr("update dynamic config", writeDynamic(s.Port, false))
		setActive(s.Port)
		fmt.Println("switched to", s.Port)
		return nil
	},
}
