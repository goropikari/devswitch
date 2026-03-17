package devswitch

import (
	"os"
	"path/filepath"
	"strings"
)

// findDotGit walks up from dir searching for a .git entry and returns its path.
func findDotGit(dir string) (string, bool) {
	for {
		p := filepath.Join(dir, ".git")
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// resolveWorktreeGitDir resolves a linked-worktree "gitdir: ..." .git file to an absolute path.
func resolveWorktreeGitDir(dotgitFile, repoRoot string) (string, error) {
	//nolint:gosec // G304: Reading .git file path derived from trusted local git repo structure
	data, err := os.ReadFile(dotgitFile)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(line, "gitdir: ") {
		return "", os.ErrNotExist
	}
	p := strings.TrimPrefix(line, "gitdir: ")
	if !filepath.IsAbs(p) {
		p = filepath.Join(repoRoot, p)
	}
	return filepath.Clean(p), nil
}

// 現在のリポジトリの git common dir を返す。worktree でも同一リポジトリで同じ値になる。
func gitCommonDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dotgit, ok := findDotGit(wd)
	if !ok {
		return "", os.ErrNotExist
	}

	info, err := os.Stat(dotgit)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		// Regular repo or main worktree: .git is a directory.
		return filepath.Clean(dotgit), nil
	}

	// Linked worktree: .git is a file pointing to .git/worktrees/<name>.
	worktreeDir, err := resolveWorktreeGitDir(dotgit, filepath.Dir(dotgit))
	if err != nil {
		return "", err
	}
	// .git/worktrees/<name>/commondir contains the relative path back to the common .git dir.
	//nolint:gosec // G304: Reading git commondir file from trusted local repo structure
	commonDirData, err := os.ReadFile(filepath.Join(worktreeDir, "commondir"))
	if err != nil {
		return "", err
	}
	common := strings.TrimSpace(string(commonDirData))
	if !filepath.IsAbs(common) {
		common = filepath.Join(worktreeDir, common)
	}
	return filepath.Clean(common), nil
}

// 現在ディレクトリの branch 名を返す。取得できない場合は "-"。
func currentBranchName() string {
	wd, err := os.Getwd()
	if err != nil {
		return "-"
	}
	dotgit, ok := findDotGit(wd)
	if !ok {
		return "-"
	}

	var headPath string
	info, err := os.Stat(dotgit)
	if err != nil {
		return "-"
	}

	if info.IsDir() {
		headPath = filepath.Join(dotgit, "HEAD")
	} else {
		// Linked worktree: HEAD lives inside the worktree-specific gitdir.
		worktreeDir, err := resolveWorktreeGitDir(dotgit, filepath.Dir(dotgit))
		if err != nil {
			return "-"
		}
		headPath = filepath.Join(worktreeDir, "HEAD")
	}

	//nolint:gosec // G304: Reading git HEAD file from trusted local repo structure
	headData, err := os.ReadFile(headPath)
	if err != nil {
		return "-"
	}
	head := strings.TrimSpace(string(headData))
	if strings.HasPrefix(head, "ref: refs/heads/") {
		return strings.TrimPrefix(head, "ref: refs/heads/")
	}
	if len(head) >= 7 {
		return head[:7]
	}
	return head
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
