package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitOps: checkpoint/rollback over the bundled MinGit (runtime\git).
// Mode 1 (workspace has .git): reuse the user's repository with private refs
//   refs/win7-agent/checkpoints/<task>/<seq>; a temporary index file is used so
//   the user's staged state is never touched; rollback restores the worktree
//   via `git restore --source` and never moves the user's branch.
// Mode 2 (no .git): private checkpoint repository under the agent's data dir,
//   driven with --git-dir/--work-tree; rollback uses reset --hard (agent-owned repo).
type gitOps struct {
	gitExe    string
	workspace string
	mode      int
	ckDir     string
	taskID    string
	seq       int
}

func newGitOps(exeDir, workspace, taskID string) (*gitOps, error) {
	gitExe := filepath.Join(exeDir, "runtime", "git", "cmd", "git.exe")
	if _, err := os.Stat(gitExe); err != nil {
		return nil, fmt.Errorf("bundled git not found at %s (product runtime incomplete)", gitExe)
	}
	g := &gitOps{gitExe: gitExe, workspace: workspace, taskID: taskID}
	if st, err := os.Stat(filepath.Join(workspace, ".git")); err == nil && st.IsDir() {
		g.mode = 1
	} else {
		g.mode = 2
		g.ckDir = filepath.Join(exeDir, "data", "workspaces", wsID(workspace), "checkpoint.git")
		if _, err := os.Stat(g.ckDir); os.IsNotExist(err) {
			if err := g.gitInitBare(); err != nil {
				// remove a possibly partial directory from a failed init, retry once
				os.RemoveAll(g.ckDir)
				if err := g.gitInitBare(); err != nil {
					return nil, fmt.Errorf("init checkpoint repo failed: %v", err)
				}
			}
		}
	}
	return g, nil
}

// gitInitBare: bare init must run without the mode-2 --git-dir/--work-tree prefix.
func (g *gitOps) gitInitBare() error {
	cmd := exec.Command(g.gitExe, "init", "--bare", g.ckDir)
	cmd.Env = g.baseEnv(nil)
	b, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, b)
	}
	return nil
}

func wsID(ws string) string {
	r := strings.NewReplacer(`\`, "_", ":", "_", "/", "_", " ", "_", ".", "_")
	s := r.Replace(ws)
	if len(s) > 60 {
		s = s[len(s)-60:]
	}
	return s
}

func (g *gitOps) baseEnv(extra map[string]string) []string {
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=win7-agent", "GIT_AUTHOR_EMAIL=agent@win7.local",
		"GIT_COMMITTER_NAME=win7-agent", "GIT_COMMITTER_EMAIL=agent@win7.local",
	)
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

// gitRun: empty extraEnv means the default index; dir "" runs in workspace.
func (g *gitOps) gitRun(indexFile string, args ...string) (string, error) {
	full := []string{g.gitExe}
	if g.mode == 2 {
		full = append(full, "--git-dir="+g.ckDir, "--work-tree="+g.workspace)
	}
	full = append(full, args...)
	cmd := exec.Command(full[0], full[1:]...)
	cmd.Dir = g.workspace
	env := map[string]string{}
	if indexFile != "" {
		env["GIT_INDEX_FILE"] = indexFile
	}
	cmd.Env = g.baseEnv(env)
	b, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

// checkpoint ref namespaces: pulse7 is the new primary; win7-agent is the
// legacy namespace whose checkpoints must remain discoverable for rollback.
const (
	refPrefixNew = "refs/pulse7/checkpoints/"
	refPrefixOld = "refs/win7-agent/checkpoints/"
)

func (g *gitOps) ref(seq int) string {
	return fmt.Sprintf("%s%s/%d", refPrefixNew, g.taskID, seq)
}

func (g *gitOps) Checkpoint() (string, error) {
	g.seq++
	ref := g.ref(g.seq)
	var indexFile string
	if g.mode == 1 {
		// temp index: never disturb the user's staged state
		indexFile = filepath.Join(filepath.Dir(g.gitExe), "..", "..", "..", "data", "sessions",
			fmt.Sprintf("index-%s-%d.tmp", g.taskID, g.seq))
		g.gitRun(indexFile, "read-tree", "HEAD") // may fail on empty repo: ignore
	}
	if _, err := g.gitRun(indexFile, "add", "-A", "--", "."); err != nil {
		return "", fmt.Errorf("git add: %v", err)
	}
	tree, err := g.gitRun(indexFile, "write-tree")
	if err != nil {
		return "", fmt.Errorf("write-tree: %v", err)
	}
	parent, perr := g.gitRun("", "rev-parse", g.ref(g.seq-1))
	if perr != nil || g.seq == 1 {
		if p, e := g.gitRun("", "rev-parse", "HEAD"); e == nil {
			parent = p
		} else {
			parent = ""
		}
	}
	commitArgs := []string{"commit-tree", tree, "-m", "win7-agent checkpoint " + ref}
	if parent != "" {
		commitArgs = append(commitArgs, "-p", parent)
	}
	commit, err := g.gitRun("", commitArgs...)
	if err != nil {
		return "", fmt.Errorf("commit-tree: %v", err)
	}
	if _, err := g.gitRun("", "update-ref", ref, commit); err != nil {
		return "", fmt.Errorf("update-ref: %v", err)
	}
	dirty, _ := g.gitRun(indexFile, "status", "--porcelain")
	n := 0
	if dirty != "" {
		n = len(strings.Split(dirty, "\n"))
	}
	mode := "user-repo+private-ref"
	if g.mode == 2 {
		mode = "private-checkpoint-repo"
	}
	return fmt.Sprintf("checkpoint %s/%d (%s, mode=%s, tree=%s, dirty-files=%d)", g.taskID, g.seq, short(commit), mode, short(tree), n), nil
}

// refs returns all persisted checkpoint refs from BOTH namespaces (pulse7
// primary + win7-agent legacy). refnames contain no spaces.
func (g *gitOps) refs() ([]string, error) {
	out, err := g.gitRun("", "for-each-ref", "--format=%(refname)", refPrefixNew)
	if err != nil {
		return nil, err
	}
	list := strings.Fields(out)
	oldOut, _ := g.gitRun("", "for-each-ref", "--format=%(refname)", refPrefixOld)
	list = append(list, strings.Fields(oldOut)...)
	return list, nil
}

// Rollback: restore worktree to the given (or latest persisted) checkpoint —
// scan-based so PREVIOUS sessions' checkpoints are usable. Never moves the
// user's branch in mode 1; agent-created files are cleaned via the manifest.
func (g *gitOps) Rollback(toSeq int) (string, error) {
	list, err := g.refs()
	if err != nil || len(list) == 0 {
		return "", fmt.Errorf("no checkpoints found under refs/pulse7/ or refs/win7-agent/ (%v)", err)
	}
	target := list[len(list)-1]
	if toSeq > 0 {
		want := g.ref(toSeq)
		found := false
		for _, r := range list {
			found = found || r == want
		}
		if !found {
			return "", fmt.Errorf("checkpoint %s not found (available: %d, latest %s)", want, len(list), target)
		}
		target = want
	}
	if g.mode == 1 {
		if out, err := g.gitRun("", "restore", "--source="+target, "--staged", "--worktree", "--", "."); err != nil {
			return "", fmt.Errorf("git restore: %v: %s", err, out)
		}
	} else if out, err := g.gitRun("", "reset", "--hard", target); err != nil {
		return "", fmt.Errorf("git reset: %v: %s", err, out)
	}
	return fmt.Sprintf("rolled back to %s; worktree restored (checkpoints available: %d)", target, len(list)), nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
