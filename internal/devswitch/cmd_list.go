package devswitch

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list running app processes",
	Long: `Show all app processes registered in the current workspace.
The active backend (currently receiving proxy traffic) is marked with *.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, _ := loadServers()
		active := currentActive()

		if listJSON {
			type jsonEntry struct {
				Label   string `json:"label"`
				Branch  string `json:"branch"`
				Port    int    `json:"port"`
				PID     int    `json:"pid"`
				Active  bool   `json:"active"`
				Command string `json:"command"`
			}
			entries := make([]jsonEntry, 0, len(servers))
			for _, s := range servers {
				entries = append(entries, jsonEntry{
					Label:   s.Label,
					Branch:  s.Branch,
					Port:    s.Port,
					PID:     s.PID,
					Active:  s.Port == active,
					Command: s.Command,
				})
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		}

		fmt.Printf("%-22s %-17s %-8s %-8s %-6s %s\n", "LABEL", "BRANCH", "PORT", "PID", "ACTIVE", "CMD")
		for _, s := range servers {
			branch := formatBranchLabel(s.Branch)
			runCmd := s.Command
			if runCmd == "" {
				runCmd = "-"
			}
			mark := ""
			if s.Port == active {
				mark = "*"
			}
			fmt.Printf("%-22s %-17s %-8d %-8d %-6s %s\n", s.Label, branch, s.Port, s.PID, mark, runCmd)
		}
		return nil
	},
}
