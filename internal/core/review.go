package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pomelohq/pomelo/internal/services"
)

// ReviewGet returns the authored review artifact for a workspace, or {"exists":false}
// when none has been written yet. Reviews are workspace-scoped (a workspace spans
// several repos), so anchors inside are repo-qualified. Stored at the project root
// under .pom/reviews/<branch>.json.
func (s *Server) ReviewGet(branch string, isMain bool) []byte {
	empty := []byte(`{"exists":false}`)
	root := s.WorkspaceRoot
	if root == "" {
		return empty
	}
	name := services.BranchSafe(branch)
	if name == "" {
		return empty
	}
	b, err := os.ReadFile(filepath.Join(root, ".pom", "reviews", name+".json"))
	if err != nil {
		return empty
	}
	return b
}

// FilePeek returns the working-tree contents of repo/path in a workspace so a code
// anchor can show a peek without leaving the app. Path is confined to the worktree.
func (s *Server) FilePeek(branch, repo, path string, isMain bool) []byte {
	if strings.Contains(path, "..") || filepath.IsAbs(path) {
		return []byte(`{"error":"bad path"}`)
	}
	wt := repoWorktreePath(s.WorkspaceRoot, repo, branch, isMain)
	b, err := os.ReadFile(filepath.Join(wt, path))
	if err != nil {
		return []byte(`{"error":"not found"}`)
	}
	out, _ := json.Marshal(map[string]any{"repo": repo, "path": path, "content": string(b)})
	return out
}
