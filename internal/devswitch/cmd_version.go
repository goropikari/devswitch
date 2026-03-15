package devswitch

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version はビルド時に -ldflags で注入される。
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(Version)
	},
}
