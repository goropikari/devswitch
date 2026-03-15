package provider

import (
	"os"
	"os/exec"
	"strings"
	"text/template"
)

type traefikProxy struct{ env Env }

func (p traefikProxy) Name() string    { return Traefik }
func (p traefikProxy) LogPath() string { return p.env.LogFilePath }

func (p traefikProxy) Start(opts StartOptions) (StartResult, error) {
	if err := p.env.EnsureTmpDir(); err != nil {
		return StartResult{}, err
	}

	// 初回起動時は dynamic 設定の雛形を作成する。
	if _, err := os.Stat(p.env.DynConfigPath); err != nil {
		p.env.WarnErr("write initial dynamic config",
			os.WriteFile(p.env.DynConfigPath, []byte(p.env.DynInitial), 0644))
	}

	// static 設定テンプレートへ値を埋め込む。
	bindHost := p.env.BindHost
	if bindHost == "" {
		bindHost = "localhost"
	}
	conf := p.env.StaticTemplate
	conf = strings.ReplaceAll(conf, "${BIND_HOST}", bindHost)
	conf = strings.ReplaceAll(conf, "${PORT}", p.env.ListenPort)
	conf = strings.ReplaceAll(conf, "${DYNAMIC_CONFIG}", p.env.DynConfigPath)
	p.env.WarnErr("write static config",
		os.WriteFile(p.env.StaticConfigPath, []byte(conf), 0644))

	c := exec.Command("traefik", "--configFile", p.env.StaticConfigPath)
	if opts.Daemon {
		return startProcessDaemon(c, p.env)
	}

	return StartResult{}, startProcessForeground(c)
}

func (p traefikProxy) Stop() error {
	return stopProxy(p.env)
}

func (p traefikProxy) UpdateRoute(port int, grpc bool) error {
	if err := p.env.EnsureTmpDir(); err != nil {
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

	tmpl, err := template.New("dynamic").Parse(p.env.DynTemplate)
	if err != nil {
		return err
	}

	tmp := p.env.DynConfigPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, Data{Port: port, Scheme: scheme}); err != nil {
		return err
	}

	return os.Rename(tmp, p.env.DynConfigPath)
}
