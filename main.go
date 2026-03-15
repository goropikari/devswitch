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
	"text/template"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

type Server struct {
	Port int
	PID  int
}

// start-server の起動オプションを受け取るフラグ変数。
var portEnv string
var portArg string
var grpcMode bool
var proxyDaemon bool

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
// Git ルートが取れる場合は Git ルート基準、取れなければ実行ディレクトリ基準にする。
func workspaceKey() string {
	base := "unknown"

	if repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true}); err == nil {
		if wt, wtErr := repo.Worktree(); wtErr == nil {
			root := strings.TrimSpace(wt.Filesystem.Root())
			if root != "" {
				base = filepath.Clean(root)
			}
		}
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
func listenPort() string {
	p := os.Getenv("DEVSWITCH_PORT")
	if p == "" {
		p = "9000"
	}
	return p
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
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}

		port, _ := strconv.Atoi(fields[0])
		pid, _ := strconv.Atoi(fields[1])
		if !pidAlive(pid) {
			continue
		}

		servers = append(servers, Server{Port: port, PID: pid})
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
		_, _ = fmt.Fprintf(f, "%d %d\n", s.Port, s.PID)
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
func writeDynamic(port int, grpc bool) error {
	if err := ensureTmpDir(); err != nil {
		return err
	}

	scheme := "http"
	if grpc {
		scheme = "h2c"
	}

	type Data struct {
		Port   int
		Scheme string
	}

	tmpl, err := template.New("dynamic").Parse(dynamicTemplate)
	if err != nil {
		return err
	}

	tmp := dynamicPath() + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, Data{Port: port, Scheme: scheme}); err != nil {
		return err
	}

	return os.Rename(tmp, dynamicPath())
}

// promptui でサーバーを選択させる。
func selectServer(servers []Server) (Server, error) {
	if len(servers) == 0 {
		return Server{}, fmt.Errorf("no servers available")
	}

	items := make([]string, 0, len(servers))
	for _, s := range servers {
		items = append(items, fmt.Sprintf("port=%d pid=%d", s.Port, s.PID))
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

var rootCmd = &cobra.Command{Use: "devswitch"}

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
	startCmd.Flags().StringVar(&portEnv, "port-env", "", "")
	startCmd.Flags().StringVar(&portArg, "port-arg", "", "")
	startCmd.Flags().BoolVar(&grpcMode, "grpc", false, "")
	proxyCmd.Flags().BoolVar(&proxyDaemon, "daemon", true, "")

	rootCmd.AddCommand(proxyCmd)
	rootCmd.AddCommand(proxyStopCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(switchCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(cleanupCmd)

	_ = rootCmd.Execute()
}
