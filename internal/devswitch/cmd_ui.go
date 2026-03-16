package devswitch

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed ui/index.html
var uiFS embed.FS

// serveUI starts the embedded HTTP UI server on the specified port.
func serveUI(port string) error {
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
		if _, err := w.Write(data); err != nil {
			logJSON("write UI index response", "path=/", err)
		}
	})

	http.HandleFunc("/api/servers", handleGetServers)
	http.HandleFunc("/api/activate", handlePostActivate)
	http.HandleFunc("/api/stop", handlePostStop)
	http.HandleFunc("/api/start", handlePostStart)
	http.HandleFunc("/api/register", handlePostRegister)

	url := fmt.Sprintf("http://localhost:%s", port)
	fmt.Printf("UI started at %s\n", url)
	return http.ListenAndServe(":"+port, nil)
}

var uiServeCmd = &cobra.Command{
	Use:    "__ui-serve",
	Hidden: true,
	Short:  "start web ui",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveUI(uiPort)
	},
}

// lookupPortPID returns the PID of the process listening on the given TCP port
// by reading /proc/net/tcp{,6} and /proc/<pid>/fd/ without external commands.
func lookupPortPID(port int) int {
	inode := findSocketInode(port)
	if inode == "" {
		return 0
	}
	return findPIDByInode(inode)
}

// findSocketInode looks up the socket inode for a LISTEN entry on port.
func findSocketInode(port int) string {
	hexPort := fmt.Sprintf("%04X", port)
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}
			// local_address: IPADDR:PORT (hex); st 0A = TCP_LISTEN
			parts := strings.SplitN(fields[1], ":", 2)
			if len(parts) == 2 && parts[1] == hexPort && fields[3] == "0A" {
				return fields[9] // inode
			}
		}
	}
	return ""
}

// findPIDByInode scans /proc/<pid>/fd symlinks to match the socket inode.
func findPIDByInode(inode string) int {
	target := "socket:[" + inode + "]"
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name()))
			if err != nil {
				continue
			}
			if link == target {
				return pid
			}
		}
	}
	return 0
}

func handleGetServers(w http.ResponseWriter, r *http.Request) {
	servers, _ := loadServers()
	active := currentActive()
	// PID field shadows Server.PID in JSON output so we can override it.
	type serverStatus struct {
		Server
		Running bool `json:"running"`
		PID     int  `json:"PID"`
	}
	statuses := make([]serverStatus, 0, len(servers))
	for _, s := range servers {
		running := false
		if s.PID > 0 {
			running = pidAlive(s.PID)
		} else {
			running = portAlive(s.Port)
		}
		resolvedPID := s.PID
		if s.PID == 0 && running {
			resolvedPID = lookupPortPID(s.Port)
		}
		statuses = append(statuses, serverStatus{
			Server:  s,
			Running: running,
			PID:     resolvedPID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"servers": statuses,
		"active":  active,
		"proxy": map[string]interface{}{
			"port": listenPort(),
		},
	}); err != nil {
		logJSON("encode servers response", "path=/api/servers", err)
	}
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

	if s.PID == 0 {
		if !portAlive(port) {
			http.Error(w, "external server is not listening on its registered port", http.StatusBadRequest)
			return
		}
	} else if !pidAlive(s.PID) {
		// If a managed server is not running, restart it.
		newPort, err := StartAppServer(StartAppParams{
			Label:   s.Label,
			Command: s.Command,
			Args:    s.Args,
			PortEnv: s.PortEnv,
			PortArg: s.PortArg,
		})
		if err != nil {
			http.Error(w, "failed to restart server: "+err.Error(), http.StatusInternalServerError)
			return
		}
		port = newPort
	}

	if err := updateProxyRoute(port); err != nil {
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
		Label   string `json:"label"`
		Command string `json:"command"`
		PortEnv string `json:"portEnv"`
		PortArg string `json:"portArg"`
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
		Label:   req.Label,
		Command: parts[0],
		Args:    parts[1:],
		PortEnv: req.PortEnv,
		PortArg: req.PortArg,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handlePostRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label string `json:"label"`
		Port  int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !proxyAlive() {
		http.Error(w, "proxy server is not running; run `devswitch proxy start` first", http.StatusBadRequest)
		return
	}

	if req.Port <= 0 || req.Port > 65535 {
		http.Error(w, "valid port is required", http.StatusBadRequest)
		return
	}

	if !portAlive(req.Port) {
		http.Error(w, fmt.Sprintf("port %d is not listening", req.Port), http.StatusBadRequest)
		return
	}

	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = randomName()
	}

	servers, _ := loadServers()
	for _, s := range servers {
		if s.Port == req.Port {
			if err := updateProxyRoute(req.Port); err != nil {
				logJSON("update proxy route (already registered)", fmt.Sprintf("port=%d", req.Port), err)
			}
			setActive(req.Port)
			w.WriteHeader(http.StatusOK)
			return
		}
		if s.Label == label {
			http.Error(w, fmt.Sprintf("label %q is already used by port %d", label, s.Port), http.StatusBadRequest)
			return
		}
	}

	if err := addServer(Server{
		Port:    req.Port,
		PID:     0,
		Branch:  currentBranchName(),
		Label:   label,
		Command: "external",
	}); err != nil {
		logJSON("register external server", fmt.Sprintf("port=%d, label=%q", req.Port, label), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := updateProxyRoute(req.Port); err != nil {
		logJSON("update proxy route (register)", fmt.Sprintf("port=%d", req.Port), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	setActive(req.Port)
	w.WriteHeader(http.StatusOK)
}
