package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var appLabel string

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "manage app processes",
	Long:  `Start and stop app processes that run behind the reverse proxy.`,
}

var appStartCmd = &cobra.Command{
	Use:   "start [flags] -- <command> [args...]",
	Short: "start an app process",
	Long: `Start an app process with an automatically assigned free port.

devswitch picks a free port and passes it to the app via --port-env or --port-arg.
The proxy is updated to route traffic to the new app immediately.

Examples:
  devswitch app start --port-env PORT -- python -m http.server
    => PORT=54321 python -m http.server

  devswitch app start --port-arg --port -- ./myapp
    => ./myapp --port 54321

  devswitch app start --port-env PORT --grpc -- ./grpc-server
    => PORT=54321 ./grpc-server  (with gRPC/h2c routing)

  devswitch app start --label my-feature --port-env PORT -- ./myapp
    => PORT=54321 ./myapp  (label: my-feature)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !proxyAlive() {
			return fmt.Errorf("proxy server is not running; run `devswitch proxy start` first")
		}

		if len(args) == 0 {
			return fmt.Errorf("command required")
		}

		// ラベルの重複チェック（起動前に行う）。
		label := strings.TrimSpace(appLabel)
		if label != "" {
			existing, _ := loadServers()
			for _, s := range existing {
				if s.Label == label {
					return fmt.Errorf("label %q is already used by port %d (pid %d)", label, s.Port, s.PID)
				}
			}
		}

		// portEnv が有効な環境変数名かチェックする。
		if portEnv != "" {
			for _, ch := range portEnv {
				if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_') {
					return fmt.Errorf("--port-env: invalid environment variable name: %q", portEnv)
				}
			}
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
		if label == "" {
			label = randomName()
		}
		warnErr("register started server", addServer(Server{Port: port, PID: c.Process.Pid, Branch: currentBranchName(), GRPC: grpcMode, Label: label, Command: runCommand}))
		warnErr("update proxy route", updateProxyRoute(port, grpcMode))
		setActive(port)

		fmt.Println("started server", port)
		return nil
	},
}

var appStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "stop an app process",
	Long:  `Interactively select a running app process and stop it by sending SIGKILL to its TCP port.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, _ := loadServers()
		s, err := selectServer(servers)
		if err != nil {
			return err
		}

		wasActive := s.Port == currentActive()

		if err := exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", s.Port)).Run(); err != nil {
			warnErr(fmt.Sprintf("kill tcp port=%d", s.Port), err)
		}
		fmt.Println("stopped", s.Port)

		// 停止した app が active だった場合、残っている別の app へ自動的に切り替える。
		if wasActive {
			remaining, _ := loadServers()
			// fuser -k 直後はまだ PID が生存していることがあるため、停止したポートを除外する。
			var others []Server
			for _, r := range remaining {
				if r.Port != s.Port {
					others = append(others, r)
				}
			}
			if len(others) > 0 {
				next := others[len(others)-1]
				warnErr("update proxy route", updateProxyRoute(next.Port, next.GRPC))
				setActive(next.Port)
				fmt.Printf("switched active to port %d (%s, branch %s)\n", next.Port, next.Label, formatBranchLabel(next.Branch))
			} else {
				setActive(0)
				fmt.Println("no remaining app processes")
			}
		}

		return nil
	},
}
