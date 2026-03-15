package main

import (
	"bufio"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

type Server struct {
	Port    int
	PID     int
	Branch  string
	GRPC    bool
	Label   string
	Command string
}

// start-server の起動オプションを受け取るフラグ変数。
var portEnv string
var portArg string
var grpcMode bool
var proxyDaemon bool
var proxyProvider string
var proxyBindHost string

//go:embed templates/traefik_static.yml
var staticTemplate string

//go:embed templates/dynamic_initial.yml
var dynamicInitial string

//go:embed templates/dynamic_service.yml
var dynamicTemplate string

// 一時ディレクトリを決定する。
// DEVSWITCH_TMPDIR があればそれを優先し、なければランタイム固定値を使う。
func devswitchDir() string {
	dir := os.Getenv("DEVSWITCH_TMPDIR")
	if dir != "" {
		return dir
	}

	runtimeDir, err := initRuntimeDir(false)
	if err == nil {
		return runtimeDir
	}

	return filepath.Join(os.TempDir(), "devswitch-fallback")
}

// state file 名に使うキーを返す。
// Git common dir が取れる場合はそれを基準にし、取れなければ実行ディレクトリ基準にする。
func workspaceKey() string {
	base := "unknown"

	if commonDir, err := gitCommonDir(); err == nil {
		base = filepath.Clean(commonDir)
	} else if wd, wdErr := os.Getwd(); wdErr == nil {
		base = filepath.Clean(wd)
	}

	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:8])
}

// ランタイム固定の tmp dir パスを保存する state file の場所。
func runtimeDirStateFile() string {
	return filepath.Join(os.TempDir(), "devswitch-dir-"+workspaceKey())
}

// tmp dir の初期化。
// forceNew=false の場合は既存 state を再利用し、true の場合は必ず新規作成する。
func initRuntimeDir(forceNew bool) (string, error) {
	stateFile := runtimeDirStateFile()

	if !forceNew {
		if b, err := os.ReadFile(stateFile); err == nil {
			saved := strings.TrimSpace(string(b))
			if saved != "" {
				return saved, nil
			}
		}
	}

	randomDir, err := os.MkdirTemp(os.TempDir(), "devswitch-")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(stateFile, []byte(randomDir), 0644); err != nil {
		return "", err
	}

	return randomDir, nil
}

// 実際に利用する tmp dir を作成しておく。
func ensureTmpDir() error {
	return os.MkdirAll(devswitchDir(), 0755)
}

// 管理ファイルの配置先ヘルパー。
func registryFilePath() string {
	return devswitchDir() + "/devswitch_servers"
}

func activeFilePath() string {
	return devswitchDir() + "/devswitch_active"
}

func dynamicPath() string {
	return devswitchDir() + "/devswitch_dynamic.yml"
}

func staticFilePath() string {
	return devswitchDir() + "/devswitch_static.yml"
}

func proxyPIDFilePath() string {
	return devswitchDir() + "/proxy.pid"
}

func proxyLogFilePath() string {
	return devswitchDir() + "/proxy.log"
}

// proxy の待受ポートを返す。
// DEVSWITCH_PORT は数値 (1-65535) であることを検証し、不正な場合はデフォルト値を使う。
func listenPort() string {
	p := os.Getenv("DEVSWITCH_PORT")
	if p != "" {
		if n, err := strconv.Atoi(p); err == nil && n >= 1 && n <= 65535 {
			return p
		}
	}
	return "9000"
}

// proxy の待受ポートに接続できるかで、proxy 起動中かを判定する。
func proxyAlive() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:"+listenPort(), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// OS に空きポートを割り当てさせて取得する。
func freePort() int {
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// PID が生存しているかを判定する。
func pidAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// レジストリからサーバー一覧を読み込む。
// 生きていない PID は除外し、読み込み後にレジストリを自動クリーンアップする。
func loadServers() ([]Server, error) {
	file, err := os.Open(registryFilePath())
	if err != nil {
		return nil, nil
	}
	defer file.Close()

	var servers []Server
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		meta := line
		command := ""
		if parts := strings.SplitN(line, "\t", 2); len(parts) == 2 {
			meta = strings.TrimSpace(parts[0])
			command = strings.TrimSpace(parts[1])
		}

		fields := strings.Fields(meta)
		if len(fields) < 2 {
			continue
		}

		port, _ := strconv.Atoi(fields[0])
		pid, _ := strconv.Atoi(fields[1])
		branch := "-"
		grpc := false
		label := "-"
		if len(fields) >= 3 {
			branch = fields[2]
		}
		if len(fields) >= 4 {
			if parsed, err := strconv.ParseBool(fields[3]); err == nil {
				grpc = parsed
			}
		}
		if len(fields) >= 5 {
			label = fields[4]
		}
		if !pidAlive(pid) {
			continue
		}

		servers = append(servers, Server{Port: port, PID: pid, Branch: branch, GRPC: grpc, Label: label, Command: command})
	}

	_ = saveRegistry(servers)
	return servers, nil
}

// サーバー一覧をレジストリに保存する（tmp ファイル経由で原子的に更新）。
func saveRegistry(servers []Server) error {
	if err := ensureTmpDir(); err != nil {
		return err
	}

	tmp := registryFilePath() + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, s := range servers {
		_, _ = fmt.Fprintf(f, "%d %d %s %t %s", s.Port, s.PID, s.Branch, s.GRPC, s.Label)
		if strings.TrimSpace(s.Command) != "" {
			_, _ = fmt.Fprintf(f, "\t%s", s.Command)
		}
		_, _ = fmt.Fprintln(f)
	}

	return os.Rename(tmp, registryFilePath())
}

// 新規サーバーをレジストリに追加する。
func addServer(s Server) error {
	servers, _ := loadServers()
	servers = append(servers, s)
	return saveRegistry(servers)
}

// 現在アクティブなポートを書き込む。
func setActive(port int) {
	if port == 0 {
		_ = os.Remove(activeFilePath())
		return
	}
	_ = ensureTmpDir()
	_ = os.WriteFile(activeFilePath(), []byte(strconv.Itoa(port)), 0644)
}

// 現在アクティブなポートを読み取る。
func currentActive() int {
	data, err := os.ReadFile(activeFilePath())
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return p
}

// Traefik dynamic 設定を対象ポート向けに更新する。
// promptui でサーバーを選択させる。
func selectServer(servers []Server) (Server, error) {
	if len(servers) == 0 {
		return Server{}, fmt.Errorf("no servers available")
	}

	items := make([]string, 0, len(servers))
	for _, s := range servers {
		runCmd := s.Command
		if strings.TrimSpace(runCmd) == "" {
			runCmd = "-"
		}
		items = append(items, fmt.Sprintf("%-22s branch=%s port=%d pid=%d cmd=%s", s.Label, formatBranchLabel(s.Branch), s.Port, s.PID, runCmd))
	}

	prompt := promptui.Select{
		Label: "Select server",
		Items: items,
		Size:  10,
		Searcher: func(input string, index int) bool {
			needle := strings.ToLower(strings.ReplaceAll(input, " ", ""))
			haystack := strings.ToLower(strings.ReplaceAll(items[index], " ", ""))
			return strings.Contains(haystack, needle)
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return Server{}, err
	}

	return servers[idx], nil
}

var rootCmd = &cobra.Command{
	Use:          "devswitch",
	Short:        "dev server switcher with reverse proxy",
	SilenceUsage: true,
	Long: `devswitch - dev server switcher with reverse proxy

Environment variables:
  DEVSWITCH_PORT            proxy listen port (default: 9000)            [proxy start, app start, info]
  DEVSWITCH_BIND_HOST       proxy bind host   (default: localhost)       [proxy start, info]
  DEVSWITCH_PROXY_PROVIDER  proxy provider    (native|traefik|socat, default: native) [proxy start, info]
  DEVSWITCH_TMPDIR          override runtime directory path              [all commands]
`,
}

// TCP 接続可否でポート生存を判定する。
func portAlive(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// dynamic 設定を空にしてルーティングを無効化する。
func writeEmptyDynamic() error {
	if err := ensureTmpDir(); err != nil {
		return err
	}

	y := `
http:
  routers: {}
  services: {}

`

	return os.WriteFile(dynamicPath(), []byte(y), 0644)
}

func main() {
	// ルートコマンドへサブコマンドを登録して実行する。
	appStartCmd.Flags().StringVar(&portEnv, "port-env", "", "environment variable name to pass the port to the app (e.g. PORT)")
	appStartCmd.Flags().StringVar(&portArg, "port-arg", "", "flag name to pass the port as a CLI argument (e.g. --port)")
	appStartCmd.Flags().BoolVar(&grpcMode, "grpc", false, "treat the app as a gRPC server")
	appStartCmd.Flags().StringVarP(&appLabel, "label", "l", "", "label for this app process (default: random name)")
	appCmd.AddCommand(appStartCmd)
	appCmd.AddCommand(appStopCmd)
	proxyStartCmd.Flags().BoolVar(&proxyDaemon, "daemon", true, "")
	proxyStartCmd.Flags().StringVar(&proxyProvider, "provider", "", "reverse proxy provider (native|traefik|socat)")
	proxyStartCmd.Flags().StringVarP(&proxyBindHost, "bind", "b", "", "bind host (default: localhost)")
	proxyCmd.AddCommand(proxyStartCmd)
	proxyCmd.AddCommand(proxyStopCmd)

	rootCmd.AddCommand(proxyCmd)
	rootCmd.AddCommand(proxyServeCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(appCmd)
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(switchCmd)
	rootCmd.AddCommand(cleanupCmd)

	_ = rootCmd.Execute()
}
