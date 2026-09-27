package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupRestoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	omp := filepath.Join(home, ".omp")
	write(t, filepath.Join(omp, "mcp.json"), "root-mcp")
	write(t, filepath.Join(omp, "agent", "mcp.json"), "agent-mcp")
	write(t, filepath.Join(omp, "skills", "how", "SKILL.md"), "skill")
	write(t, filepath.Join(omp, "skills", "repo", ".git", "HEAD"), "ref")
	write(t, filepath.Join(omp, "agent", "agent.db"), "secret-db")

	snap := filepath.Join(home, "snaps")
	var stdout, stderr bytes.Buffer
	code := Run(Env{
		Args:     []string{"backup", "omp"},
		Stdout:   &stdout,
		Stderr:   &stderr,
		Now:      func() time.Time { return time.Date(2026, 9, 26, 16, 30, 0, 0, time.UTC) },
		OMPHome:  omp,
		SnapDir:  snap,
		UserHome: home,
	})
	if code != CodeOK {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "credential material") {
		t.Fatalf("stderr %s", stderr.String())
	}
	archivePath := strings.TrimSpace(stdout.String())
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(snap)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o700 {
		t.Fatalf("snap dir mode %o", fi.Mode().Perm())
	}

	// Collision, no force: original bytes stay.
	var errb bytes.Buffer
	code = Run(Env{
		Args:    []string{"restore", archivePath},
		Stdout:  &bytes.Buffer{},
		Stderr:  &errb,
		OMPHome: omp,
	})
	if code != CodeUsage {
		t.Fatalf("code %d", code)
	}
	if got := read(t, filepath.Join(omp, "mcp.json")); got != "root-mcp" {
		t.Fatalf("overwritten %q", got)
	}

	// Force replaces archived paths only.
	write(t, filepath.Join(omp, "mcp.json"), "changed")
	write(t, filepath.Join(omp, "notes.txt"), "keep")
	code = Run(Env{
		Args:    []string{"restore", archivePath, "--force"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		OMPHome: omp,
	})
	if code != CodeOK {
		t.Fatal(code)
	}
	if got := read(t, filepath.Join(omp, "mcp.json")); got != "root-mcp" {
		t.Fatalf("mcp %q", got)
	}
	if got := read(t, filepath.Join(omp, "agent", "mcp.json")); got != "agent-mcp" {
		t.Fatalf("agent mcp %q", got)
	}
	if _, err := os.Stat(filepath.Join(omp, "skills", "repo", ".git", "HEAD")); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(omp, "notes.txt")); got != "keep" {
		t.Fatalf("local file %q", got)
	}
	if got := read(t, filepath.Join(omp, "agent", "agent.db")); got != "secret-db" {
		t.Fatalf("db packed or changed %q", got)
	}
	if _, err := os.Stat(filepath.Join(omp, "skills", "repo", ".git", "config")); err == nil {
		// .git/HEAD existed before restore and must remain; .git/config was never a source.
	}

	var listOut bytes.Buffer
	code = Run(Env{
		Args:   []string{"list", archivePath},
		Stdout: &listOut,
		Stderr: &bytes.Buffer{},
	})
	if code != CodeOK {
		t.Fatal(code)
	}
	text := listOut.String()
	if !strings.Contains(text, "agent omp") || !strings.Contains(text, "payload/omp/mcp.json") {
		t.Fatalf("list:\n%s", text)
	}
	if strings.Contains(text, ".git") || strings.Contains(text, "agent.db") {
		t.Fatalf("junk in list:\n%s", text)
	}
}

func TestOutputInsideHomeRefused(t *testing.T) {
	home := t.TempDir()
	omp := filepath.Join(home, ".omp")
	write(t, filepath.Join(omp, "mcp.json"), "x")
	out := filepath.Join(omp, "snap.tar.gz")
	var stderr bytes.Buffer
	code := Run(Env{
		Args:    []string{"backup", "omp", "-o", out},
		Stdout:  &bytes.Buffer{},
		Stderr:  &stderr,
		OMPHome: omp,
		Now:     time.Now,
	})
	if code != CodeUsage {
		t.Fatalf("code %d stderr %s", code, stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("archive written inside home")
	}
}

func TestRestoreForceRefusesDirectory(t *testing.T) {
	home := t.TempDir()
	omp := filepath.Join(home, ".omp")
	write(t, filepath.Join(omp, "mcp.json"), "x")
	snap := filepath.Join(home, "snaps")
	var stdout bytes.Buffer
	code := Run(Env{
		Args:    []string{"backup", "omp"},
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		OMPHome: omp,
		SnapDir: snap,
		Now:     func() time.Time { return time.Date(2026, 9, 26, 1, 2, 3, 0, time.UTC) },
	})
	if code != CodeOK {
		t.Fatal(code)
	}
	os.Remove(filepath.Join(omp, "mcp.json"))
	if err := os.Mkdir(filepath.Join(omp, "mcp.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(omp, "mcp.json", "keep")
	write(t, marker, "stay")
	code = Run(Env{
		Args:    []string{"restore", strings.TrimSpace(stdout.String()), "--force"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		OMPHome: omp,
	})
	if code != CodeUsage {
		t.Fatalf("code %d", code)
	}
	if got := read(t, marker); got != "stay" {
		t.Fatalf("wrote into directory collision: %q", got)
	}
}
func TestRestoreRefusesSymlinkDest(t *testing.T) {
	home := t.TempDir()
	omp := filepath.Join(home, ".omp")
	write(t, filepath.Join(omp, "mcp.json"), "x")
	var stdout bytes.Buffer
	code := Run(Env{
		Args:    []string{"backup", "omp"},
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		OMPHome: omp,
		SnapDir: filepath.Join(home, "snaps"),
		Now:     func() time.Time { return time.Unix(1, 0).UTC() },
	})
	if code != CodeOK {
		t.Fatal(code)
	}
	outside := filepath.Join(home, "outside.txt")
	write(t, outside, "keep")
	os.Remove(filepath.Join(omp, "mcp.json"))
	if err := os.Symlink(outside, filepath.Join(omp, "mcp.json")); err != nil {
		t.Fatal(err)
	}
	code = Run(Env{
		Args:    []string{"restore", strings.TrimSpace(stdout.String()), "--force"},
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
		OMPHome: omp,
	})
	if code != CodeUsage {
		t.Fatalf("code %d", code)
	}
	if got := read(t, outside); got != "keep" {
		t.Fatalf("symlink followed: %q", got)
	}
}

func TestGitAndSymlinkSkipped(t *testing.T) {
	home := t.TempDir()
	omp := filepath.Join(home, ".omp")
	write(t, filepath.Join(omp, "mcp.json"), "x")
	skill := filepath.Join(omp, "skills", "how", "SKILL.md")
	write(t, skill, "s")
	if err := os.Symlink(skill, filepath.Join(omp, "skills", "alias.md")); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	code := Run(Env{
		Args:    []string{"backup", "omp"},
		Stdout:  &stdout,
		Stderr:  &bytes.Buffer{},
		OMPHome: omp,
		SnapDir: filepath.Join(home, "snaps"),
		Now:     func() time.Time { return time.Unix(0, 0).UTC() },
	})
	if code != CodeOK {
		t.Fatal(code)
	}
	var listOut bytes.Buffer
	code = Run(Env{
		Args:   []string{"list", strings.TrimSpace(stdout.String())},
		Stdout: &listOut,
		Stderr: &bytes.Buffer{},
	})
	if code != CodeOK {
		t.Fatal(code)
	}
	if strings.Contains(listOut.String(), "alias.md") {
		t.Fatalf("symlink packed:\n%s", listOut.String())
	}
}

func write(t *testing.T, abs, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, abs string) string {
	t.Helper()
	b, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
