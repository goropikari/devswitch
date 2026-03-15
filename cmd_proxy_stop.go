package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop proxy daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		portTarget := fmt.Sprintf("%s/tcp", listenPort())

		pidData, err := os.ReadFile(proxyPIDFilePath())
		if err != nil {
			// PID ファイルが無くても待受ポートを閉じに行く。
			if err := exec.Command("fuser", "-k", portTarget).Run(); err != nil {
				warnErr("kill proxy listen port", err)
			}
			fmt.Println("proxy stop requested", listenPort())
			return cleanupAllStartedServers()
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
		if err != nil {
			return fmt.Errorf("invalid proxy pid: %w", err)
		}

		if !pidAlive(pid) {
			if err := os.Remove(proxyPIDFilePath()); err != nil && !os.IsNotExist(err) {
				warnErr("remove proxy pid file", err)
			}
			if err := exec.Command("fuser", "-k", portTarget).Run(); err != nil {
				warnErr("kill proxy listen port", err)
			}
			fmt.Println("proxy stop requested", listenPort())
			return cleanupAllStartedServers()
		}

		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			return err
		}

		if err := exec.Command("fuser", "-k", portTarget).Run(); err != nil {
			warnErr("kill proxy listen port", err)
		}

		if err := os.Remove(proxyPIDFilePath()); err != nil && !os.IsNotExist(err) {
			warnErr("remove proxy pid file", err)
		}
		fmt.Println("proxy stopped", pid)
		return cleanupAllStartedServers()
	},
}
