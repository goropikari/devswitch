package main

import (
	"github.com/goropikari/devswitch/provider"
	"github.com/spf13/cobra"
)

// proxyServeCmd は native proxy サーバー本体を起動する隠しコマンド。
var proxyServeCmd = &cobra.Command{
	Use:    "__proxy-serve",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return provider.RunServer(buildProxyEnv())
	},
}
