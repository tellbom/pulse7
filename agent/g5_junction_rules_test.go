package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestJunctionWriteEditRulesMatchFinalTargetOnly(t *testing.T) {
	for _, tool := range []string{"write", "edit"} {
		for _, finalRule := range []bool{true, false} {
			name := tool + "/alias-rule"
			if finalRule {
				name = tool + "/final-rule"
			}
			t.Run(name, func(t *testing.T) {
				r, _, workspace, _ := newPermissionTestRegistry(t, "open", nil, "")
				outside := filepath.Join(t.TempDir(), "target")
				if err := os.MkdirAll(outside, 0755); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(outside, "file.txt")
				if err := os.WriteFile(target, []byte("before"), 0644); err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(workspace, "alias")
				if b, err := exec.Command("cmd.exe", "/c", "mklink", "/J", alias, outside).CombinedOutput(); err != nil {
					t.Fatalf("junction: %s %v", b, err)
				}
				requested := filepath.Join(alias, "file.txt")
				pattern := requested
				if finalRule {
					pattern = target
				}
				r.permissions.Rules = []permissionRule{{Tool: tool, Pattern: pattern, Action: "deny"}}
				readArgs, _ := json.Marshal(map[string]string{"path": requested})
				if _, err := r.toolRead(string(readArgs)); err != nil {
					t.Fatal(err)
				}
				args := map[string]string{"path": requested, "content": "after", "old_string": "before", "new_string": "after"}
				raw, _ := json.Marshal(args)
				result := r.Execute(tool, string(raw))
				t.Logf("pattern=%s requested=%s result=%s", pattern, requested, result)
				disk, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				if finalRule {
					if !strings.HasPrefix(result, "error:") || string(disk) != "before" {
						t.Fatalf("final deny did not protect file: %s %q", result, disk)
					}
				} else {
					if strings.HasPrefix(result, "error:") || string(disk) != "after" {
						t.Fatalf("alias rule unexpectedly matched: %s %q", result, disk)
					}
				}
			})
		}
	}
}
