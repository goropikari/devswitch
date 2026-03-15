package devswitch

import (
	"os"
	"strings"

	"github.com/goropikari/devswitch/internal/provider"
)

func providerStateFilePath() string {
	return devswitchDir() + "/proxy.provider"
}

func normalizeProxyProviderName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return provider.Native
	}
	return n
}

func readProxyProviderName() string {
	b, err := os.ReadFile(providerStateFilePath())
	if err != nil {
		return ""
	}
	return normalizeProxyProviderName(string(b))
}

func writeProxyProviderName(name string) error {
	if err := ensureTmpDir(); err != nil {
		return err
	}
	return os.WriteFile(providerStateFilePath(), []byte(normalizeProxyProviderName(name)), 0644)
}

func resolveCurrentProxyProviderName() string {
	if fromState := readProxyProviderName(); fromState != "" {
		return fromState
	}
	if fromEnv := normalizeProxyProviderName(os.Getenv("DEVSWITCH_PROXY_PROVIDER")); fromEnv != "" {
		return fromEnv
	}
	return provider.Native
}

func resolveBindHost() string {
	if proxyBindHost != "" {
		return proxyBindHost
	}
	return os.Getenv("DEVSWITCH_BIND_HOST")
}

func buildProxyEnv() provider.Env {
	exe, _ := os.Executable()
	return provider.Env{
		BindHost:         resolveBindHost(),
		ListenPort:       listenPort(),
		LogFilePath:      proxyLogFilePath(),
		PIDFilePath:      proxyPIDFilePath(),
		DynConfigPath:    dynamicPath(),
		StaticConfigPath: staticFilePath(),
		DynTemplate:      dynamicTemplate,
		DynInitial:       dynamicInitial,
		StaticTemplate:   staticTemplate,
		Executable:       exe,
		GetActive:        currentActive,
		SetActive:        setActive,
		EnsureTmpDir:     ensureTmpDir,
		PIDAlive:         pidAlive,
		WarnErr:          warnErr,
	}
}

func newReverseProxy(name string) (ReverseProxy, error) {
	p, err := provider.New(name, buildProxyEnv())
	if err != nil {
		return nil, err
	}
	rp, ok := p.(ReverseProxy)
	if !ok {
		return nil, os.ErrInvalid
	}
	return rp, nil
}

func currentReverseProxy() (ReverseProxy, error) {
	return newReverseProxy(resolveCurrentProxyProviderName())
}

func updateProxyRoute(port int, grpc bool) error {
	p, err := currentReverseProxy()
	if err != nil {
		return err
	}
	return p.UpdateRoute(port, grpc)
}
