package devswitch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var appLabel string
var registerLabel string

type StartAppParams struct {
	Label   string
	Command string
	Args    []string
	PortEnv string
	PortArg string
}

func StartAppServer(params StartAppParams) (int, error) {
	if !proxyAlive() {
		return 0, fmt.Errorf("proxy server is not running; run `devswitch proxy start` first")
	}

	if params.Command == "" {
		return 0, fmt.Errorf("command required")
	}

	if params.PortEnv == "" && params.PortArg == "" {
		return 0, fmt.Errorf("either --port-env or --port-arg must be specified to pass the free port to the app")
	}

	// ラベルの重複チェック（起動前に行う）。
	label := strings.TrimSpace(params.Label)
	if label != "" {
		existing, _ := loadServers()
		var nextServers []Server
		for _, s := range existing {
			if s.Label == label {
				if pidAlive(s.PID) {
					return 0, fmt.Errorf("label %q is already used by port %d (pid %d)", label, s.Port, s.PID)
				}
				// If not alive, we will effectively replace it by NOT adding it to nextServers
				continue
			}
			nextServers = append(nextServers, s)
		}
		// Clear out dead entry with same label before starting new
		if err := saveRegistry(nextServers); err != nil {
			logJSON("clear dead server entries", fmt.Sprintf("label=%q", label), err)
		}
	}

	// portEnv が有効な環境変数名かチェックする。
	if params.PortEnv != "" {
		for _, ch := range params.PortEnv {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_') {
				return 0, fmt.Errorf("--port-env: invalid environment variable name: %q", params.PortEnv)
			}
		}
	}

	port := freePort()
	command := params.Command
	// Copy args to avoid mutating the original params.Args
	commandArgs := make([]string, len(params.Args))
	copy(commandArgs, params.Args)

	if params.PortArg != "" {
		commandArgs = append(commandArgs, params.PortArg, strconv.Itoa(port))
	}

	// コマンドのパス解決と存在確認
	exePath, err := exec.LookPath(params.Command)
	if err != nil {
		return 0, fmt.Errorf("command not found: %q", params.Command)
	}

	// 簡易的な禁止リスト
	forbidden := map[string]bool{
		"rm":       true,
		"mkfs":     true,
		"dd":       true,
		"shutdown": true,
		"reboot":   true,
	}
	if forbidden[filepath.Base(exePath)] {
		return 0, fmt.Errorf("command %q is not allowed for security reasons", params.Command)
	}

	// 指定されたコマンドを空きポート付きで起動する。
	//nolint:gosec // G204: executing user-supplied command is the core purpose; validated via LookPath and forbidden-list above
	c := exec.Command(exePath, commandArgs...)
	env := os.Environ()
	if params.PortEnv != "" {
		env = append(env, fmt.Sprintf("%s=%d", params.PortEnv, port))
	}

	c.Env = env
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	if err := c.Start(); err != nil {
		return 0, err
	}

	// 起動後にレジストリ・ルーティング・active を更新する。
	if label == "" {
		label = randomName()
	}
	if err := addServer(Server{
		Port:    port,
		PID:     c.Process.Pid,
		Branch:  currentBranchName(),
		Label:   label,
		Command: command,     // Base command
		Args:    params.Args, // Base args
		PortEnv: params.PortEnv,
		PortArg: params.PortArg,
	}); err != nil {
		logJSON("register started server", fmt.Sprintf("port=%d, label=%q", port, label), err)
	}
	if err := updateProxyRoute(port); err != nil {
		logJSON("update proxy route", fmt.Sprintf("port=%d", port), err)
	}
	setActive(port)

	return port, nil
}

func StopAppServer(port int) error {
	servers, _ := loadServers()
	var s *Server
	var sIdx = -1
	for i, svr := range servers {
		if svr.Port == port {
			s = &svr
			sIdx = i
			break
		}
	}
	if s == nil {
		return fmt.Errorf("server not found for port %d", port)
	}

	wasActive := s.Port == currentActive()

	// Kill the process if managed.
	if s.PID > 0 {
		p, err := os.FindProcess(s.PID)
		if err == nil {
			if err := p.Kill(); err != nil {
				logJSON("kill managed process", fmt.Sprintf("port=%d, pid=%d", s.Port, s.PID), err)
			}
		} else {
			logJSON("find managed process", fmt.Sprintf("port=%d, pid=%d", s.Port, s.PID), err)
		}
		// Fallback to lookup port PID if process kill fails.
		if pid := lookupPortPID(s.Port); pid > 0 {
			if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
				logJSON("kill port process", fmt.Sprintf("port=%d, resolved_pid=%d", s.Port, pid), err)
			}
		}
	}

	// Mark as stopped in registry.
	if sIdx != -1 {
		// Remove from registry so it is no longer managed/proxied.
		servers = append(servers[:sIdx], servers[sIdx+1:]...)
		if err := saveRegistry(servers); err != nil {
			logJSON("remove stopped server from registry", fmt.Sprintf("port=%d", s.Port), err)
		}
	}

	// 停止した app が active だった場合、残っている別の app へ自動的に切り替える。
	if wasActive {
		remaining, _ := loadServers()
		var others []Server
		for _, r := range remaining {
			if r.Port != s.Port {
				others = append(others, r)
			}
		}
		if len(others) > 0 {
			next := others[len(others)-1]
			if err := updateProxyRoute(next.Port); err != nil {
				logJSON("update proxy route after stop", fmt.Sprintf("port=%d", next.Port), err)
			}
			setActive(next.Port)
			fmt.Printf("switched active to port %d (%s, branch %s)\n", next.Port, next.Label, formatBranchLabel(next.Branch))
		} else {
			setActive(0)
			fmt.Println("no remaining running app processes")
		}
	}

	return nil
}

var appRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "register an existing app process",
	Long:  `Register an already running app process (e.g. started manually) to devswitch.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !proxyAlive() {
			return fmt.Errorf("proxy server is not running; run `devswitch proxy start` first")
		}

		if registerPort == 0 {
			return fmt.Errorf("--port is required")
		}

		if !portAlive(registerPort) {
			return fmt.Errorf("port %d is not listening", registerPort)
		}

		label := strings.TrimSpace(registerLabel)
		if label == "" {
			label = randomName()
		}

		// Validate uniqueness by port/label and clean dead entries.
		existing, _ := loadServers()
		for _, s := range existing {
			if s.Port == registerPort {
				if err := updateProxyRoute(registerPort); err != nil {
					logJSON("update proxy route for existing register", fmt.Sprintf("port=%d", registerPort), err)
				}
				setActive(registerPort)
				fmt.Printf("port %d is already registered (label: %s)\n", registerPort, s.Label)
				fmt.Printf("switched active to port %d\n", registerPort)
				return nil
			}
			if s.Label == label {
				return fmt.Errorf("label %q is already used by port %d", label, s.Port)
			}
		}

		s := Server{
			Port:    registerPort,
			PID:     0, // 0 indicates external process
			Branch:  currentBranchName(),
			Label:   label,
			Command: "external",
			Args:    nil,
		}

		if err := addServer(s); err != nil {
			return err
		}

		if err := updateProxyRoute(registerPort); err != nil {
			return err
		}
		setActive(registerPort)

		fmt.Printf("registered external app on port %d (label: %s)\n", registerPort, label)
		return nil
	},
}

var registerPort int

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
		if len(args) == 0 {
			return fmt.Errorf("command required")
		}

		port, err := StartAppServer(StartAppParams{
			Label:   appLabel,
			Command: args[0],
			Args:    args[1:],
			PortEnv: portEnv,
			PortArg: portArg,
		})
		if err != nil {
			return err
		}

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

		if err := StopAppServer(s.Port); err != nil {
			return err
		}
		fmt.Println("stopped", s.Port)
		return nil
	},
}
