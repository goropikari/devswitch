package devswitch

import (
	"fmt"
	"os"
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
				logJSON(fmt.Sprintf("send SIGTERM to pid=%d", s.PID), "", err)
			}
		}

		// Fallback to lookup port PID if port is still listening.
		if pid := lookupPortPID(s.Port); pid > 0 {
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
				logJSON(fmt.Sprintf("send SIGKILL to resolved pid=%d", pid), fmt.Sprintf("port=%d", s.Port), err)
			}
		}
		fmt.Println("stopped server", s.Port)
		stopped++
	}

	if err := saveRegistry(nil); err != nil {
		logJSON("save empty registry", "", err)
	}
	if err := writeEmptyDynamic(); err != nil {
		logJSON("write empty dynamic config", "", err)
	}
	if err := os.Remove(activeFilePath()); err != nil && !os.IsNotExist(err) {
		logJSON("remove active file", "", err)
	}

	fmt.Println("cleanup completed", stopped)
	return nil
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "stop all app processes and reset state",
	Long:  `Send SIGTERM to all registered app processes, release their ports, and reset the registry and active state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cleanupAllStartedServers()
	},
}
