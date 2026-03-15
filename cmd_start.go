package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use: "start-server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !proxyAlive() {
			return fmt.Errorf("proxy server is not running; run `devswitch proxy start` first")
		}

		if len(args) == 0 {
			return fmt.Errorf("command required")
		}

		port := freePort()
		command := args[0]
		commandArgs := args[1:]

		if portArg != "" {
			commandArgs = append(commandArgs, portArg, strconv.Itoa(port))
		}

		// 指定されたコマンドを空きポート付きで起動する。
		c := exec.Command(command, commandArgs...)
		env := os.Environ()
		if portEnv != "" {
			env = append(env, fmt.Sprintf("%s=%d", portEnv, port))
		}

		c.Env = env
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Start(); err != nil {
			return err
		}

		runCommand := strings.Join(append([]string{command}, commandArgs...), " ")
		if portEnv != "" {
			runCommand = fmt.Sprintf("%s=%d %s", portEnv, port, runCommand)
		}

		// 起動後にレジストリ・ルーティング・active を更新する。
		warnErr("register started server", addServer(Server{Port: port, PID: c.Process.Pid, Branch: currentBranchName(), GRPC: grpcMode, Command: runCommand}))
		warnErr("update proxy route", updateProxyRoute(port, grpcMode))
		setActive(port)

		fmt.Println("started server", port)
		return nil
	},
}
