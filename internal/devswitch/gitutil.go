package devswitch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 現在のリポジトリの git common dir を返す。worktree でも同一リポジトリで同じ値になる。
func gitCommonDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	common := strings.TrimSpace(string(out))
	if common == "" {
		return "", os.ErrNotExist
	}

	if filepath.IsAbs(common) {
		return filepath.Clean(common), nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.Join(wd, common)), nil
}

// 現在ディレクトリの branch 名を返す。取得できない場合は "-"。
func currentBranchName() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "-"
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "-"
	}
	return branch
}

// branch 表示用: 最大 15 文字に省略し、未設定時は [-] で返す。
func formatBranchLabel(branch string) string {
	b := strings.TrimSpace(branch)
	if b == "" || b == "-" {
		return "[-]"
	}
	if len(b) > 15 {
		b = b[:12] + "..."
	}
	return "[" + b + "]"
}
