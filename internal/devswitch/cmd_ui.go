package devswitch

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed ui/index.html
var uiFS embed.FS

var uiPort string

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "start web ui",
	RunE: func(cmd *cobra.Command, args []string) error {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			data, err := uiFS.ReadFile("ui/index.html")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write(data)
		})

		http.HandleFunc("/api/servers", handleGetServers)
		http.HandleFunc("/api/activate", handlePostActivate)
		http.HandleFunc("/api/stop", handlePostStop)
		http.HandleFunc("/api/start", handlePostStart)

		url := fmt.Sprintf("http://localhost:%s", uiPort)
		fmt.Printf("UI started at %s\n", url)
		openBrowser(url)

		return http.ListenAndServe(":"+uiPort, nil)
	},
}

func handleGetServers(w http.ResponseWriter, r *http.Request) {
	servers, _ := loadServers()
	active := currentActive()
	type serverStatus struct {
		Server
		Running bool `json:"running"`
	}
	statuses := make([]serverStatus, 0, len(servers))
	for _, s := range servers {
		statuses = append(statuses, serverStatus{
			Server:  s,
			Running: s.PID > 0 && pidAlive(s.PID),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"servers": statuses,
		"active":  active,
	})
}

func handlePostActivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	servers, _ := loadServers()
	var s *Server
	for _, svr := range servers {
		if svr.Label == req.Label {
			s = &svr
			break
		}
	}
	if s == nil {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}

	port := s.Port
	grpc := s.GRPC

	// If the server is not running, restart it.
	if s.PID == 0 || !pidAlive(s.PID) {
		newPort, err := StartAppServer(StartAppParams{
			Label:    s.Label,
			Command:  s.Command,
			Args:     s.Args,
			PortEnv:  s.PortEnv,
			PortArg:  s.PortArg,
			GRPCMode: s.GRPC,
		})
		if err != nil {
			http.Error(w, "failed to restart server: "+err.Error(), http.StatusInternalServerError)
			return
		}
		port = newPort
	}

	if err := updateProxyRoute(port, grpc); err != nil {
		http.Error(w, "failed to update proxy route: "+err.Error(), http.StatusInternalServerError)
		return
	}
	setActive(port)
	w.WriteHeader(http.StatusOK)
}

func handlePostStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port int `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := StopAppServer(req.Port); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handlePostStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label    string `json:"label"`
		Command  string `json:"command"`
		PortEnv  string `json:"portEnv"`
		PortArg  string `json:"portArg"`
		GRPC     bool   `json:"grpc"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	parts := strings.Fields(req.Command)
	if len(parts) == 0 {
		http.Error(w, "command required", http.StatusBadRequest)
		return
	}

	_, err := StartAppServer(StartAppParams{
		Label:    req.Label,
		Command:  parts[0],
		Args:     parts[1:],
		PortEnv:  req.PortEnv,
		PortArg:  req.PortArg,
		GRPCMode: req.GRPC,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
	}
}
