package devswitch

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version はビルド時に -ldflags で注入される。
// 注入されていない場合は runtime/debug からモジュールバージョンを読む。
var Version = "dev"

func resolveVersion() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return Version
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(resolveVersion())
	},
}
