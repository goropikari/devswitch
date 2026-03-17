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
	uiPID, _ := readIntFile(uiPIDFilePath())
	uiPort, _ := readIntFile(uiPortFilePath())

	stoppedByPID := false
	if uiPID > 0 {
		if err := syscall.Kill(uiPID, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("signal ui pid %d: %w", uiPID, err)
		}
		stoppedByPID = true
		fmt.Println("ui stop requested", uiPID)
	}

	if uiPort > 0 {
		if pid := lookupPortPID(uiPort); pid > 0 && (!stoppedByPID || pid != uiPID) {
			if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
				return fmt.Errorf("signal ui port pid %d (port %d): %w", pid, uiPort, err)
			}
			fmt.Println("ui stop requested by port", uiPort, pid)
		}
	}

	if err := os.Remove(uiPIDFilePath()); err != nil && !os.IsNotExist(err) {
		logJSON("remove UI pid file", uiPIDFilePath(), err)
	}
	if err := os.Remove(uiPortFilePath()); err != nil && !os.IsNotExist(err) {
		logJSON("remove UI port file", uiPortFilePath(), err)
	}
	return nil
}

func readIntFile(path string) (int, error) {
	//nolint:gosec // G304: callers always pass internal devswitch state file paths
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	v := strings.TrimSpace(string(b))
	if v == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("parse int from %s: %w", path, err)
	}
	if parsed <= 0 {
		return 0, nil
	}
	return parsed, nil
}
