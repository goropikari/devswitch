package devswitch

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop the reverse proxy daemon",
	Long:  `Stop the running reverse proxy and terminate all registered app processes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
			warnErr("remove tmpdir", err)
		} else {
			fmt.Println("removed tmpdir", dir)
		}
		return nil
	},
}
