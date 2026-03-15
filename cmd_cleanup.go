package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

func cleanupAllStartedServers() error {
	// 登録済みサーバーを全停止し、状態ファイルを初期化する。
	servers, err := loadServers()
	if err != nil {
		return err
	}

	stopped := 0
	for _, s := range servers {
		if s.PID > 0 && pidAlive(s.PID) {
			if err := syscall.Kill(s.PID, syscall.SIGTERM); err != nil {
				warnErr(fmt.Sprintf("send SIGTERM to pid=%d", s.PID), err)
			}
		}

		if err := exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", s.Port)).Run(); err != nil {
			warnErr(fmt.Sprintf("kill tcp port=%d", s.Port), err)
		}
		fmt.Println("stopped server", s.Port)
		stopped++
	}

	warnErr("save empty registry", saveRegistry(nil))
	warnErr("write empty dynamic config", writeEmptyDynamic())
	if err := os.Remove(activeFilePath()); err != nil && !os.IsNotExist(err) {
		warnErr("remove active file", err)
	}

	fmt.Println("cleanup completed", stopped)
	return nil
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "stop all started servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cleanupAllStartedServers()
	},
}
