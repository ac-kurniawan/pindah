package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectOMPSkipsJunkAndGit(t *testing.T) {
	home := t.TempDir()
	must := func(rel, body string) {
		t.Helper()
		abs := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must("mcp.json", "root")
	must("agent/mcp.json", "agent")
	must("agent/config.yml", "cfg")
	must("agent/agent.db", "nope")
	must("agent/extensions/orca.ts", "ext")
	must("agent/extensions/nested/nope.ts", "nope")
	must("skills/pstack/how/SKILL.md", "skill")
	must("skills/clone/.git/config", "git")
	must("skills/clone/SKILL.md", "cloned")
	must("plugins/installed_plugins.json", "{}")
	must("plugins/cache/plugin.json", "cache")
	must("logs/omp.log", "log")
	must("autoqa.db", "db")

	link := filepath.Join(home, "skills", "link.md")
	if err := os.Symlink(filepath.Join(home, "mcp.json"), link); err != nil {
		t.Fatal(err)
	}

	got, err := CollectOMP(home)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"mcp.json":                       "root",
		"agent/mcp.json":                 "agent",
		"agent/config.yml":               "cfg",
		"agent/extensions/orca.ts":       "ext",
		"skills/pstack/how/SKILL.md":     "skill",
		"skills/clone/SKILL.md":          "cloned",
		"plugins/installed_plugins.json": "{}",
	}
	if len(got) != len(want) {
		rels := make([]string, len(got))
		for i, s := range got {
			rels[i] = s.Rel
		}
		t.Fatalf("got %d files %v", len(got), rels)
	}
	for _, s := range got {
		body, ok := want[s.Rel]
		if !ok {
			t.Fatalf("unexpected %s", s.Rel)
		}
		if s.HomeID != "omp" {
			t.Fatalf("home %s", s.HomeID)
		}
		b, err := os.ReadFile(s.Abs)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != body {
			t.Fatalf("%s body %s", s.Rel, b)
		}
	}
}

func TestCollectOpenCode(t *testing.T) {
	cfg := t.TempDir()
	skills := t.TempDir()
	share := t.TempDir()
	write := func(root, rel string) {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(cfg, "opencode.jsonc")
	write(cfg, "plugins/crg-plugin.ts")
	write(cfg, "plugins/orca.js")
	write(cfg, "plugins/skip.txt")
	write(cfg, "profiles/default/AGENTS.md")
	write(cfg, "skills/pstack/how/SKILL.md")
	write(cfg, "skills/repo/.git/HEAD")
	write(cfg, "node_modules/pkg/index.js")
	write(skills, "skills/design/SKILL.md")
	write(skills, "bin/opencode")
	write(skills, "node_modules/pkg/index.js")
	write(share, "auth.json")
	write(share, "opencode.db")
	write(share, "log/out.log")

	got, err := CollectOpenCode(cfg, skills, share)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"opencode.jsonc":             "opencode-config",
		"plugins/crg-plugin.ts":      "opencode-config",
		"plugins/orca.js":            "opencode-config",
		"profiles/default/AGENTS.md": "opencode-config",
		"skills/pstack/how/SKILL.md": "opencode-config",
		"skills/design/SKILL.md":     "opencode-skills",
		"auth.json":                  "opencode-share",
	}
	if len(got) != len(want) {
		rels := make([]string, len(got))
		for i, s := range got {
			rels[i] = s.HomeID + ":" + s.Rel
		}
		t.Fatalf("got %v", rels)
	}
	for _, s := range got {
		home, ok := want[s.Rel]
		if !ok || home != s.HomeID {
			t.Fatalf("unexpected %s %s", s.HomeID, s.Rel)
		}
	}
}
