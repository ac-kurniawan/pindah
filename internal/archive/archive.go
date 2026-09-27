// Package archive reads and writes a pindah snapshot.
package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

const Format = 1

// File is one payload member. Path is the tar path, starting with payload/.
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Mode   uint32 `json:"mode"`
}

// Manifest is the first member of every archive.
type Manifest struct {
	Format  int       `json:"format"`
	Agent   string    `json:"agent"`
	Created time.Time `json:"created"`
	Files   []File    `json:"files"`
}

// Source is a file to pack. Rel is the path under the home, slash-separated.
type Source struct {
	HomeID string
	Root   string
	Rel    string
	Abs    string
}

// Snapshot is a verified archive held in memory.
type Snapshot struct {
	Manifest Manifest
	Payload  map[string][]byte // tar path -> bytes
}

// HomesFor reports the home-id values an agent may store.
func HomesFor(agent string) (map[string]bool, error) {
	switch agent {
	case "omp":
		return map[string]bool{"omp": true}, nil
	case "opencode":
		return map[string]bool{
			"opencode-config": true,
			"opencode-skills": true,
			"opencode-share":  true,
		}, nil
	default:
		return nil, fmt.Errorf("unknown agent %q", agent)
	}
}

// WriteArchive packs sources into dest. dest must not already be the final
// name of a partial write: this writes dest.tmp and renames.
func WriteArchive(dest, agent string, created time.Time, sources []Source) error {
	homes, err := HomesFor(agent)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return errors.New("nothing to pack")
	}

	type packed struct {
		src  Source
		body []byte
		sum  string
		mode uint32
	}
	items := make([]packed, 0, len(sources))
	seen := map[string]bool{}
	for _, src := range sources {
		if !homes[src.HomeID] {
			return fmt.Errorf("home %q is not owned by %s", src.HomeID, agent)
		}
		rel, err := cleanRel(src.Rel)
		if err != nil {
			return fmt.Errorf("%s: %w", src.Abs, err)
		}
		tarPath := path.Join("payload", src.HomeID, rel)
		if seen[tarPath] {
			return fmt.Errorf("duplicate path %s", tarPath)
		}
		seen[tarPath] = true
		body, mode, err := readFile(src.Abs)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		items = append(items, packed{
			src:  Source{HomeID: src.HomeID, Rel: rel, Abs: src.Abs},
			body: body,
			sum:  hex.EncodeToString(sum[:]),
			mode: uint32(mode),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return path.Join("payload", items[i].src.HomeID, items[i].src.Rel) <
			path.Join("payload", items[j].src.HomeID, items[j].src.Rel)
	})

	man := Manifest{
		Format:  Format,
		Agent:   agent,
		Created: created.UTC(),
		Files:   make([]File, len(items)),
	}
	for i, it := range items {
		man.Files[i] = File{
			Path:   path.Join("payload", it.src.HomeID, it.src.Rel),
			SHA256: it.sum,
			Size:   int64(len(it.body)),
			Mode:   it.mode,
		}
	}
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	manBytes = append(manBytes, '\n')

	dir := path.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".pindah-*.tar.gz")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	gz := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gz)
	if err := writeMember(tw, "manifest.json", manBytes, 0o644); err != nil {
		return err
	}
	for i, it := range items {
		if err := writeMember(tw, man.Files[i].Path, it.body, os.FileMode(it.mode)); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return err
	}
	ok = true
	return nil
}

func writeMember(tw *tar.Writer, name string, body []byte, mode os.FileMode) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    int64(mode.Perm()),
		Size:    int64(len(body)),
		ModTime: time.Unix(0, 0).UTC(),
		Format:  tar.FormatPAX,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}

func readFile(abs string) ([]byte, os.FileMode, error) {
	fi, err := os.Lstat(abs)
	if err != nil {
		return nil, 0, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, 0, fmt.Errorf("%s is a symlink", abs)
	}
	if !fi.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("%s is not a regular file", abs)
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return nil, 0, err
	}
	return body, fi.Mode().Perm(), nil
}

// ReadArchive loads and verifies a snapshot.
func ReadArchive(r io.Reader) (*Snapshot, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	var man *Manifest
	var manCount int
	payload := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("%s is not a regular file", hdr.Name)
		}
		name, err := cleanMember(hdr.Name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", hdr.Name, err)
		}
		if hdr.Size < 0 || hdr.Size > 64<<20 {
			return nil, fmt.Errorf("%s: size %d rejected", name, hdr.Size)
		}
		body, err := io.ReadAll(io.LimitReader(tr, hdr.Size+1))
		if err != nil {
			return nil, err
		}
		if int64(len(body)) != hdr.Size {
			return nil, fmt.Errorf("%s: size mismatch", name)
		}
		if name == "manifest.json" {
			manCount++
			if manCount > 1 {
				return nil, errors.New("duplicate manifest.json")
			}
			parsed, err := parseManifest(body)
			if err != nil {
				return nil, err
			}
			man = parsed
			continue
		}
		if _, ok := payload[name]; ok {
			return nil, fmt.Errorf("duplicate member %s", name)
		}
		payload[name] = body
	}
	if man == nil {
		return nil, errors.New("manifest.json missing")
	}
	if err := checkPayload(man, payload); err != nil {
		return nil, err
	}
	return &Snapshot{Manifest: *man, Payload: payload}, nil
}

func parseManifest(body []byte) (*Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var man Manifest
	if err := dec.Decode(&man); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("manifest: trailing data")
	}
	if man.Format != Format {
		return nil, fmt.Errorf("format %d rejected", man.Format)
	}
	homes, err := HomesFor(man.Agent)
	if err != nil {
		return nil, err
	}
	if man.Created.IsZero() {
		return nil, errors.New("manifest: created is zero")
	}
	if man.Files == nil {
		return nil, errors.New("manifest: files missing")
	}
	seen := map[string]bool{}
	for i, f := range man.Files {
		p, err := cleanMember(f.Path)
		if err != nil {
			return nil, fmt.Errorf("files[%d]: %w", i, err)
		}
		if p != f.Path {
			return nil, fmt.Errorf("files[%d]: path not clean", i)
		}
		home, rel, ok := strings.Cut(strings.TrimPrefix(p, "payload/"), "/")
		if !ok || !strings.HasPrefix(p, "payload/") || rel == "" || !homes[home] {
			return nil, fmt.Errorf("files[%d]: path %s not in an owned home", i, f.Path)
		}
		if _, err := cleanRel(rel); err != nil {
			return nil, fmt.Errorf("files[%d]: %w", i, err)
		}
		if seen[p] {
			return nil, fmt.Errorf("duplicate manifest path %s", p)
		}
		seen[p] = true
		if i > 0 && man.Files[i-1].Path >= f.Path {
			return nil, errors.New("manifest: files not sorted")
		}
		if f.Size < 0 {
			return nil, fmt.Errorf("files[%d]: negative size", i)
		}
		if len(f.SHA256) != 64 || !isHex(f.SHA256) {
			return nil, fmt.Errorf("files[%d]: bad sha256", i)
		}
		if f.Mode&^uint32(os.ModePerm) != 0 {
			return nil, fmt.Errorf("files[%d]: mode has non-permission bits", i)
		}
	}
	return &man, nil
}

func checkPayload(man *Manifest, payload map[string][]byte) error {
	want := map[string]File{}
	for _, f := range man.Files {
		want[f.Path] = f
	}
	for name := range payload {
		f, ok := want[name]
		if !ok {
			return fmt.Errorf("member %s not in manifest", name)
		}
		body := payload[name]
		if int64(len(body)) != f.Size {
			return fmt.Errorf("%s: size %d, manifest %d", name, len(body), f.Size)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			return fmt.Errorf("%s: sha256 mismatch", name)
		}
	}
	for name := range want {
		if _, ok := payload[name]; !ok {
			return fmt.Errorf("listed file missing: %s", name)
		}
	}
	return nil
}

func cleanMember(name string) (string, error) {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") {
		return "", errors.New("illegal member name")
	}
	c := path.Clean(name)
	if c == "." || strings.HasPrefix(c, "../") || c == ".." {
		return "", errors.New("illegal member name")
	}
	return c, nil
}

func cleanRel(rel string) (string, error) {
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "\\") {
		return "", errors.New("illegal relative path")
	}
	c := path.Clean(rel)
	if c == "." || strings.HasPrefix(c, "../") || c == ".." {
		return "", errors.New("illegal relative path")
	}
	return c, nil
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		default:
			return false
		}
	}
	return true
}

// SplitPayload returns home-id and the path relative to that home.
func SplitPayload(tarPath string) (home, rel string, err error) {
	rest, ok := strings.CutPrefix(tarPath, "payload/")
	if !ok {
		return "", "", fmt.Errorf("not a payload path: %s", tarPath)
	}
	home, rel, ok = strings.Cut(rest, "/")
	if !ok || home == "" || rel == "" {
		return "", "", fmt.Errorf("not a payload path: %s", tarPath)
	}
	return home, rel, nil
}
