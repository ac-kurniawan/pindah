package archive

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "mcp.json")
	if err := os.WriteFile(src, []byte("{\"a\":1}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "out.tar.gz")
	created := time.Date(2026, 9, 26, 16, 30, 0, 0, time.UTC)
	err := WriteArchive(dest, "omp", created, []Source{{
		HomeID: "omp",
		Rel:    "mcp.json",
		Abs:    src,
	}})
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", fi.Mode().Perm())
	}
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	snap, err := ReadArchive(f)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Manifest.Agent != "omp" || snap.Manifest.Format != 1 {
		t.Fatalf("%+v", snap.Manifest)
	}
	if !snap.Manifest.Created.Equal(created) {
		t.Fatalf("created %s", snap.Manifest.Created)
	}
	body := snap.Payload["payload/omp/mcp.json"]
	if string(body) != "{\"a\":1}\n" {
		t.Fatalf("body %q", body)
	}
	if snap.Manifest.Files[0].Mode != 0o640 {
		t.Fatalf("mode %o", snap.Manifest.Files[0].Mode)
	}
}

func TestSHA256Mismatch(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "a.txt")
	if err := os.WriteFile(src, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "out.tar.gz")
	if err := WriteArchive(dest, "omp", time.Now(), []Source{{
		HomeID: "omp", Rel: "a.txt", Abs: src,
	}}); err != nil {
		t.Fatal(err)
	}
	snap := mustRead(t, dest)
	snap.Manifest.Files[0].SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	err := checkPayload(&snap.Manifest, snap.Payload)
	if err == nil {
		t.Fatal("expected sha256 mismatch")
	}
}

func TestEmptyAndForeignHome(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "out.tar.gz")
	if err := WriteArchive(dest, "omp", time.Now(), nil); err == nil {
		t.Fatal("empty should fail")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("partial archive left behind")
	}
	src := filepath.Join(root, "a.txt")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteArchive(dest, "omp", time.Now(), []Source{{
		HomeID: "opencode-share", Rel: "auth.json", Abs: src,
	}})
	if err == nil {
		t.Fatal("foreign home should fail")
	}
}

func TestSymlinkRefused(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real.txt")
	if err := os.WriteFile(real, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	err := WriteArchive(filepath.Join(root, "out.tar.gz"), "omp", time.Now(), []Source{{
		HomeID: "omp", Rel: "link.txt", Abs: link,
	}})
	if err == nil {
		t.Fatal("symlink should fail at write")
	}
}

func TestRejectsBadManifest(t *testing.T) {
	cases := []string{
		`{"format":2,"agent":"omp","created":"2026-09-26T16:30:00Z","files":[]}`,
		`{"format":1,"agent":"claude","created":"2026-09-26T16:30:00Z","files":[]}`,
		`{"format":1,"agent":"omp","created":"2026-09-26T16:30:00Z","files":[{"path":"payload/opencode-share/auth.json","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":0,"mode":420}]}`,
		`{"format":1,"agent":"omp","created":"2026-09-26T16:30:00Z","files":[{"path":"../etc/passwd","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":0,"mode":420}]}`,
	}
	for _, c := range cases {
		_, err := parseManifest([]byte(c))
		if err == nil {
			t.Fatalf("accepted %s", c)
		}
	}
}

func mustRead(t *testing.T, dest string) *Snapshot {
	t.Helper()
	f, err := os.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	snap, err := ReadArchive(f)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}
