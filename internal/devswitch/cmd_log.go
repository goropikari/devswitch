package devswitch

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// LogEntry is a structured log entry for error events.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Action    string `json:"action"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

// logJSON outputs a structured JSON log entry to stderr.
func logJSON(action string, message string, err error) {
	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     "error",
		Action:    action,
		Message:   message,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	data, _ := json.Marshal(entry)
	fmt.Fprintf(os.Stderr, "%s\n", string(data))
}

func warnErr(action string, err error) {
	if err != nil {
		logJSON(action, "", err)
	}
}
