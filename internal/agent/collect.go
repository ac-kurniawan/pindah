package agent

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/arcaku-labs/pindah/internal/archive"
)

func files(root string, rels ...string) ([]archive.Source, error) {
	var out []archive.Source
	for _, rel := range rels {
		got, err := one(root, rel)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

func one(root, rel string) ([]archive.Source, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	fi, err := os.Lstat(abs)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, nil
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", abs)
	}
	return []archive.Source{{
		Root: root,
		Rel:  path.Clean(rel),
		Abs:  abs,
	}}, nil
}

func glob(root, dir, pattern string) ([]archive.Source, error) {
	absDir := filepath.Join(root, filepath.FromSlash(dir))
	fi, err := os.Lstat(absDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, nil
	}
	matches, err := filepath.Glob(filepath.Join(absDir, pattern))
	if err != nil {
		return nil, err
	}
	var out []archive.Source
	for _, abs := range matches {
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return nil, err
		}
		got, err := one(root, filepath.ToSlash(rel))
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return out, nil
}

// tree packs regular files under relDir, skipping symlinks and any path
// component named .git.
func tree(root, relDir string) ([]archive.Source, error) {
	absDir := filepath.Join(root, filepath.FromSlash(relDir))
	fi, err := os.Lstat(absDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, nil
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absDir)
	}
	var out []archive.Source
	err = filepath.WalkDir(absDir, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if name == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if strings.Contains(slash, "..") {
			return fmt.Errorf("path escaped root: %s", slash)
		}
		out = append(out, archive.Source{
			Root: root,
			Rel:  slash,
			Abs:  abs,
		})
		return nil
	})
	return out, err
}

func withHome(home string, srcs []archive.Source) []archive.Source {
	for i := range srcs {
		srcs[i].HomeID = home
	}
	return srcs
}
