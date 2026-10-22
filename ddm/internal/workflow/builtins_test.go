package workflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMkdocsConfigHook(t *testing.T) {
	for _, withHook := range []bool{true, false} {
		repo, system := t.TempDir(), t.TempDir()
		os.Mkdir(filepath.Join(repo, "docs"), 0o755)
		if withHook {
			os.Mkdir(filepath.Join(system, "scripts"), 0o755)
			os.WriteFile(filepath.Join(system, "scripts", "mkdocs_ddm_generated.py"), nil, 0o644)
		}
		a := BuiltinArgs{Plan: &Plan{Site: "x", Vars: Vars{Repo: repo, System: system}}, Log: &bytes.Buffer{}}
		if err := mkdocsConfig(context.Background(), a); err != nil {
			t.Fatal(err)
		}
		cfg, _ := os.ReadFile(filepath.Join(repo, "mkdocs.yml"))
		if got := strings.Contains(string(cfg), "mkdocs_ddm_generated.py"); got != withHook {
			t.Errorf("hook listed = %v, want %v:\n%s", got, withHook, cfg)
		}
	}
}
