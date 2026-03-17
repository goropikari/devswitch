package devswitch

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop the reverse proxy daemon",
	Long:  `Stop the running reverse proxy and terminate all registered app processes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := stopUIDaemon(); err != nil {
			logJSON("stop UI daemon", "", err)
		}

		proxyImpl, err := currentReverseProxy()
		if err != nil {
			return err
		}

		if err := proxyImpl.Stop(); err != nil {
			return err
		}
		fmt.Println("proxy stop requested", listenPort())
		if err := cleanupAllStartedServers(); err != nil {
			return err
		}

		dir := devswitchDir()
		if err := os.RemoveAll(dir); err != nil {
			logJSON("remove tmpdir", fmt.Sprintf("dir=%s", dir), err)
		} else {
			fmt.Println("removed tmpdir", dir)
		}
		return nil
	},
}

func stopUIDaemon() error {
	b, err := os.ReadFile(uiPIDFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	pidStr := strings.TrimSpace(string(b))
	if pidStr == "" {
		return nil
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return fmt.Errorf("parse ui pid %q: %w", pidStr, err)
	}
	if pid <= 0 {
		return nil
	}

	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("signal ui pid %d: %w", pid, err)
	}

	if err := os.Remove(uiPIDFilePath()); err != nil && !os.IsNotExist(err) {
		logJSON("remove UI pid file", uiPIDFilePath(), err)
	}
	fmt.Println("ui stop requested", pid)
	return nil
}
