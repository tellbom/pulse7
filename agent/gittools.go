package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// gitOps: checkpoint/rollback over the bundled MinGit (runtime\git).
// Mode 1 (workspace has .git): reuse the user's repository with private refs
//
//	refs/win7-agent/checkpoints/<task>/<seq>; a temporary index file is used so
//	the user's staged state is never touched; rollback restores the worktree
//	via `git restore --source` and never moves the user's branch.
//
// Mode 2 (no .git): private checkpoint repository under the agent's data dir,
//
//	driven with --git-dir/--work-tree; rollback remains path-scoped.
type gitOps struct {
	gitExe    string
	workspace string
	mode      int
	ckDir     string
	indexDir  string
	taskID    string
	seq       int
	seqLoaded bool
}

type checkpointMetadata struct {
	DirtyFiles      *int      `json:"dirty_files,omitempty"`
	DirtyFilesBasis string    `json:"dirty_files_basis,omitempty"`
	Workspace       string    `json:"workspace"`
	TaskID          string    `json:"task_id"`
	Kind            string    `json:"kind"`
	Seq             int       `json:"seq"`
	CreatedAt       time.Time `json:"created_at"`
	Commit          string    `json:"commit"`
	Tree            string    `json:"tree"`
	Ref             string    `json:"ref"`
}

func (g *gitOps) checkpointIndexPath(seq int) string {
	dir := g.indexDir
	if dir == "" {
		dir = filepath.Join(filepath.Dir(g.gitExe), "..", "..", "..", "data", "sessions")
	}
	return filepath.Join(dir, fmt.Sprintf("index-%s-%d.tmp", g.taskID, seq))
}

func (g *gitOps) checkpointMetadataPath() string {
	if g.indexDir != "" {
		return filepath.Join(g.indexDir, "checkpoint-metadata.jsonl")
	}
	exeDir := filepath.Clean(filepath.Join(filepath.Dir(g.gitExe), "..", "..", ".."))
	return filepath.Join(exeDir, "data", "workspaces", wsID(g.workspace), "checkpoint-metadata.jsonl")
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
	if err := g.loadSequence(); err != nil {
		return nil, err
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
	cmd := g.gitCommand(indexFile, args...)
	b, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(b)), err
}

func (g *gitOps) gitCommand(indexFile string, args ...string) *exec.Cmd {
	// Checkpoints are byte-preservation machinery. Do not inherit a user's
	// global autocrlf setting and silently rewrite LF/CRLF during snapshot or
	// restore operations.
	full := []string{g.gitExe, "-c", "core.autocrlf=false", "-c", "core.safecrlf=false"}
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
	return cmd
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
	return g.CheckpointAs("model")
}

func (g *gitOps) CheckpointAs(kind string) (string, error) {
	if kind != "model" && kind != "auto" {
		return "", fmt.Errorf("invalid checkpoint kind %q", kind)
	}
	if err := g.loadSequence(); err != nil {
		return "", err
	}
	n, err := g.workspaceChangedFiles()
	if err != nil {
		return "", err
	}
	nextSeq := g.seq + 1
	ref := g.ref(nextSeq)
	var indexFile string
	if g.mode == 1 {
		// temp index: never disturb the user's staged state
		indexFile = g.checkpointIndexPath(nextSeq)
		g.gitRun(indexFile, "read-tree", "HEAD") // may fail on empty repo: ignore
	}
	if _, err := g.gitRun(indexFile, "add", "-A", "--", "."); err != nil {
		return "", fmt.Errorf("git add: %v", err)
	}
	tree, err := g.gitRun(indexFile, "write-tree")
	if err != nil {
		return "", fmt.Errorf("write-tree: %v", err)
	}
	parent, perr := g.gitRun("", "rev-parse", g.ref(nextSeq-1))
	if perr != nil || nextSeq == 1 {
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
	createdAt := time.Now().UTC()
	basis := "workspace relative to HEAD (ordinary index status)"
	if g.mode == 2 {
		basis = "private checkpoint repository index/HEAD status"
	}
	meta := checkpointMetadata{
		DirtyFiles: &n, DirtyFilesBasis: basis,
		Workspace: filepath.Clean(g.workspace), TaskID: g.taskID, Kind: kind, Seq: nextSeq,
		CreatedAt: createdAt, Commit: commit, Tree: tree, Ref: ref,
	}
	if err := g.persistCheckpoint(meta); err != nil {
		g.gitRun("", "update-ref", "-d", ref)
		return "", fmt.Errorf("persist checkpoint metadata: %w", err)
	}
	g.seq = nextSeq
	mode := "user-repo+private-ref"
	if g.mode == 2 {
		mode = "private-checkpoint-repo"
	}
	return fmt.Sprintf("checkpoint %s/%d (kind=%s, %s, time=%s, mode=%s, tree=%s, 工作区相对 HEAD 的变更文件数=%d)", g.taskID, g.seq, kind, short(commit), createdAt.Format(time.RFC3339), mode, short(tree), n), nil
}

// Count the ordinary index's porcelain records, not the byte-preserving
// checkpoint index. Keep the user's Git configuration for this observation.
func (g *gitOps) workspaceChangedFiles() (int, error) {
	args := []string{}
	if g.mode == 2 {
		args = append(args, "--git-dir="+g.ckDir, "--work-tree="+g.workspace)
	}
	args = append(args, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd := exec.Command(g.gitExe, args...)
	cmd.Dir = g.workspace
	for _, entry := range g.baseEnv(nil) {
		if !strings.HasPrefix(strings.ToUpper(entry), "GIT_INDEX_FILE=") && !strings.HasPrefix(strings.ToUpper(entry), "GIT_OPTIONAL_LOCKS=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git status: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return countStatusRecords(data)
}

func countStatusRecords(data []byte) (int, error) {
	n := 0
	for len(data) > 0 {
		end := bytes.IndexByte(data, 0)
		if end < 4 || data[2] != ' ' {
			return 0, fmt.Errorf("git status: invalid porcelain record")
		}
		rename := data[0] == 'R' || data[1] == 'R' || data[0] == 'C' || data[1] == 'C'
		data = data[end+1:]
		if rename {
			end = bytes.IndexByte(data, 0)
			if end <= 0 {
				return 0, fmt.Errorf("git status: missing rename/copy source path")
			}
			data = data[end+1:]
		}
		n++
	}
	return n, nil
}

func (g *gitOps) loadSequence() error {
	if g.seqLoaded {
		return nil
	}
	refs, err := g.taskRefs()
	if err != nil {
		return fmt.Errorf("load checkpoint sequence: %w", err)
	}
	if len(refs) > 0 {
		g.seq = refs[len(refs)-1].seq
	}
	g.seqLoaded = true
	return nil
}

func (g *gitOps) persistCheckpoint(meta checkpointMetadata) error {
	path := g.checkpointMetadataPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	b, err := json.Marshal(meta)
	if err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
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

type checkpointRef struct {
	ref       string
	namespace string
	taskID    string
	seq       int
}

func parseCheckpointRef(ref string) (checkpointRef, bool) {
	for _, namespace := range []string{refPrefixNew, refPrefixOld} {
		if !strings.HasPrefix(ref, namespace) {
			continue
		}
		rest := strings.TrimPrefix(ref, namespace)
		cut := strings.LastIndex(rest, "/")
		if cut <= 0 || cut == len(rest)-1 {
			return checkpointRef{}, false
		}
		seq, err := strconv.Atoi(rest[cut+1:])
		if err != nil || seq <= 0 {
			return checkpointRef{}, false
		}
		return checkpointRef{ref: ref, namespace: namespace, taskID: rest[:cut], seq: seq}, true
	}
	return checkpointRef{}, false
}

func (g *gitOps) taskRefs() ([]checkpointRef, error) {
	refs, err := g.refs()
	if err != nil {
		return nil, err
	}
	bySeq := map[int]checkpointRef{}
	for _, ref := range refs {
		parsed, ok := parseCheckpointRef(ref)
		if !ok || parsed.taskID != g.taskID {
			continue
		}
		previous, exists := bySeq[parsed.seq]
		if !exists || (previous.namespace == refPrefixOld && parsed.namespace == refPrefixNew) {
			bySeq[parsed.seq] = parsed
		}
	}
	out := make([]checkpointRef, 0, len(bySeq))
	for _, ref := range bySeq {
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].seq < out[j].seq })
	return out, nil
}

// rollbackTarget resolves the requested checkpoint without changing either the
// worktree or the user's index. Refs are restricted to the current task and
// sorted by numeric sequence; a primary ref wins over a legacy duplicate.
func (g *gitOps) rollbackTarget(toSeq int) (checkpointMetadata, int, error) {
	list, err := g.taskRefs()
	if err != nil || len(list) == 0 {
		return checkpointMetadata{}, 0, fmt.Errorf("no checkpoints found for task %s under refs/pulse7/ or refs/win7-agent/ (%v)", g.taskID, err)
	}
	target := list[len(list)-1]
	if toSeq > 0 {
		found := false
		for _, candidate := range list {
			if candidate.seq == toSeq {
				target = candidate
				found = true
				break
			}
		}
		if !found {
			return checkpointMetadata{}, len(list), fmt.Errorf("checkpoint %s/%d not found (available: %d, latest %s)", g.taskID, toSeq, len(list), target.ref)
		}
	}
	meta, err := g.metadataForRef(target)
	if err != nil {
		return checkpointMetadata{}, len(list), err
	}
	return meta, len(list), nil
}

func (g *gitOps) metadataForRef(target checkpointRef) (checkpointMetadata, error) {
	commit, err := g.gitRun("", "rev-parse", target.ref)
	if err != nil {
		return checkpointMetadata{}, fmt.Errorf("resolve checkpoint commit %s: %w", target.ref, err)
	}
	path := g.checkpointMetadataPath()
	if f, openErr := os.Open(path); openErr == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var meta checkpointMetadata
			if err := json.Unmarshal(sc.Bytes(), &meta); err != nil {
				f.Close()
				return checkpointMetadata{}, fmt.Errorf("invalid checkpoint metadata: %w", err)
			}
			if meta.Ref == target.ref && meta.Commit == commit {
				f.Close()
				return meta, nil
			}
		}
		scanErr := sc.Err()
		f.Close()
		if scanErr != nil {
			return checkpointMetadata{}, scanErr
		}
	} else if !os.IsNotExist(openErr) {
		return checkpointMetadata{}, openErr
	}
	tree, err := g.gitRun("", "rev-parse", target.ref+"^{tree}")
	if err != nil {
		return checkpointMetadata{}, fmt.Errorf("resolve checkpoint tree %s: %w", target.ref, err)
	}
	stamp, err := g.gitRun("", "show", "-s", "--format=%cI", target.ref)
	if err != nil {
		return checkpointMetadata{}, fmt.Errorf("resolve checkpoint time %s: %w", target.ref, err)
	}
	createdAt, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return checkpointMetadata{}, fmt.Errorf("parse checkpoint time %q: %w", stamp, err)
	}
	return checkpointMetadata{
		Workspace: filepath.Clean(g.workspace), TaskID: target.taskID, Seq: target.seq,
		CreatedAt: createdAt, Commit: commit, Tree: tree, Ref: target.ref,
	}, nil
}

// rollbackPaths restores only paths recorded as Agent changes. It deliberately
// omits --staged: the user's index is outside the rollback boundary.
func (g *gitOps) rollbackPaths(target string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := []string{"restore", "--source=" + target, "--worktree", "--"}
	for _, p := range paths {
		rel, err := filepath.Rel(g.workspace, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("manifest path %s is outside workspace %s", p, g.workspace)
		}
		args = append(args, rel)
	}
	if out, err := g.gitRun("", args...); err != nil {
		return fmt.Errorf("git restore: %v: %s", err, out)
	}
	return nil
}

func (g *gitOps) targetPathSpec(target, path string) (string, error) {
	rel, err := filepath.Rel(g.workspace, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("manifest path %s is outside workspace %s", path, g.workspace)
	}
	return target + ":" + filepath.ToSlash(rel), nil
}

func (g *gitOps) targetHasPath(target, path string) (bool, error) {
	spec, err := g.targetPathSpec(target, path)
	if err != nil {
		return false, err
	}
	err = g.gitCommand("", "cat-file", "-e", spec).Run()
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, err
}

func (g *gitOps) targetBlob(target, path string) ([]byte, error) {
	spec, err := g.targetPathSpec(target, path)
	if err != nil {
		return nil, err
	}
	b, err := g.gitCommand("", "cat-file", "blob", spec).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("read checkpoint blob %s: %v: %s", spec, err, strings.TrimSpace(string(b)))
	}
	return b, nil
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
