// Package cli is the pindah command line.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/arcaku-labs/pindah/internal/agent"
	"github.com/arcaku-labs/pindah/internal/archive"
)

// Code is a process exit code.
type Code int

const (
	CodeOK    Code = 0
	CodeUsage Code = 1
	CodeIO    Code = 2
)

// Env is the process boundary. Tests pass a fake one.
type Env struct {
	Args     []string
	Stdout   io.Writer
	Stderr   io.Writer
	Now      func() time.Time
	UserHome string
	OMPHome  string
	OCConfig string
	OCSkills string
	OCShare  string
	SnapDir  string
}

// Run executes one invocation.
func Run(env Env) Code {
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Stdout == nil {
		env.Stdout = os.Stdout
	}
	if env.Stderr == nil {
		env.Stderr = os.Stderr
	}
	args := env.Args
	if len(args) == 0 {
		return usage(env, "usage: pindah backup|restore|list ...")
	}
	switch args[0] {
	case "backup":
		return backup(env, args[1:])
	case "restore":
		return restore(env, args[1:])
	case "list":
		return list(env, args[1:])
	default:
		return usage(env, "unknown command "+args[0])
	}
}

func backup(env Env, args []string) Code {
	agentName, out, err := parseBackup(args)
	if err != nil {
		return usage(env, err.Error())
	}
	homes, err := homesFor(env, agentName)
	if err != nil {
		return usage(env, err.Error())
	}
	if out == "" {
		dir, err := snapDir(env)
		if err != nil {
			return ioerr(env, err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return ioerr(env, err)
		}
		stamp := env.Now().UTC().Format("20060102T150405Z")
		out = filepath.Join(dir, "pindah-"+agentName+"-"+stamp+".tar.gz")
	} else {
		out, err = filepath.Abs(out)
		if err != nil {
			return ioerr(env, err)
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
			return ioerr(env, err)
		}
	}
	if inside, home := insideHome(out, homes); inside {
		return usage(env, fmt.Sprintf("output %s is inside %s", out, home))
	}

	var srcs []archive.Source
	switch agentName {
	case "omp":
		srcs, err = agent.CollectOMP(homes["omp"])
	case "opencode":
		srcs, err = agent.CollectOpenCode(homes["opencode-config"], homes["opencode-skills"], homes["opencode-share"])
	}
	if err != nil {
		return ioerr(env, err)
	}
	if len(srcs) == 0 {
		return usage(env, "nothing to pack")
	}
	if err := archive.WriteArchive(out, agentName, env.Now(), srcs); err != nil {
		return classifyWrite(env, err)
	}
	fmt.Fprintln(env.Stderr, "warning: archive contains credential material and is not encrypted")
	fmt.Fprintln(env.Stdout, out)
	return CodeOK
}

func restore(env Env, args []string) Code {
	path, force, err := parseRestore(args)
	if err != nil {
		return usage(env, err.Error())
	}
	snap, err := openArchive(path)
	if err != nil {
		return usage(env, err.Error())
	}
	homes, err := homesFor(env, snap.Manifest.Agent)
	if err != nil {
		return usage(env, err.Error())
	}
	type dest struct {
		file archive.File
		abs  string
		body []byte
	}
	plan := make([]dest, 0, len(snap.Manifest.Files))
	for _, f := range snap.Manifest.Files {
		homeID, rel, err := archive.SplitPayload(f.Path)
		if err != nil {
			return usage(env, err.Error())
		}
		root, ok := homes[homeID]
		if !ok {
			return usage(env, "archive home "+homeID+" is not owned by "+snap.Manifest.Agent)
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		plan = append(plan, dest{file: f, abs: abs, body: snap.Payload[f.Path]})
	}

	var collisions []string
	var blocked []string
	for _, d := range plan {
		fi, err := os.Lstat(d.abs)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return ioerr(env, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 || !fi.Mode().IsRegular() {
			blocked = append(blocked, d.abs)
			continue
		}
		collisions = append(collisions, d.abs)
	}
	if len(blocked) > 0 {
		fmt.Fprintln(env.Stderr, "cannot replace:")
		for _, p := range blocked {
			fmt.Fprintln(env.Stderr, p)
		}
		return CodeUsage
	}
	if len(collisions) > 0 && !force {
		fmt.Fprintln(env.Stderr, "destination exists:")
		for _, p := range collisions {
			fmt.Fprintln(env.Stderr, p)
		}
		return CodeUsage
	}

	// Stage every file first. A failure here removes the temps and leaves
	// the homes unchanged. The rename pass is the commit.
	type staged struct {
		tmp  string
		dest string
	}
	done := make([]staged, 0, len(plan))
	ok := false
	defer func() {
		if ok {
			return
		}
		for _, s := range done {
			os.Remove(s.tmp)
		}
	}()
	for _, d := range plan {
		parent := filepath.Dir(d.abs)
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return ioerr(env, err)
		}
		mode := os.FileMode(d.file.Mode).Perm()
		if mode == 0 {
			mode = 0o600
		}
		tmp, err := stage(parent, d.body, mode)
		if err != nil {
			return ioerr(env, err)
		}
		done = append(done, staged{tmp: tmp, dest: d.abs})
	}
	for _, s := range done {
		if err := os.Rename(s.tmp, s.dest); err != nil {
			return ioerr(env, err)
		}
	}
	ok = true
	return CodeOK
}

func list(env Env, args []string) Code {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		return usage(env, "usage: pindah list <archive>")
	}
	snap, err := openArchive(args[0])
	if err != nil {
		return usage(env, err.Error())
	}
	m := snap.Manifest
	fmt.Fprintf(env.Stdout, "format %d\nagent %s\ncreated %s\n", m.Format, m.Agent, m.Created.UTC().Format(time.RFC3339))
	for _, f := range m.Files {
		fmt.Fprintf(env.Stdout, "%04o %d %s %s\n", f.Mode, f.Size, f.SHA256, f.Path)
	}
	return CodeOK
}

func stage(dir string, body []byte, mode os.FileMode) (string, error) {
	tmp, err := os.CreateTemp(dir, ".pindah-restore-*")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		os.Remove(name)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

func openArchive(path string) (*archive.Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return archive.ReadArchive(f)
}

func parseBackup(args []string) (agentName, out string, err error) {
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-o" {
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("usage: pindah backup <agent> [-o path]")
			}
			out = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			return "", "", fmt.Errorf("unknown flag %s", a)
		}
		rest = append(rest, a)
	}
	if len(rest) != 1 {
		return "", "", fmt.Errorf("usage: pindah backup <agent> [-o path]")
	}
	if rest[0] != "omp" && rest[0] != "opencode" {
		return "", "", fmt.Errorf("unknown agent %s", rest[0])
	}
	return rest[0], out, nil
}

func parseRestore(args []string) (path string, force bool, err error) {
	var rest []string
	for _, a := range args {
		if a == "--force" {
			force = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			return "", false, fmt.Errorf("unknown flag %s", a)
		}
		rest = append(rest, a)
	}
	if len(rest) != 1 {
		return "", false, fmt.Errorf("usage: pindah restore <archive> [--force]")
	}
	return rest[0], force, nil
}

func homesFor(env Env, agentName string) (map[string]string, error) {
	switch agentName {
	case "omp":
		home := env.OMPHome
		if home == "" {
			home = filepath.Join(mustHome(env), ".omp")
		}
		return agent.OMPHomes(home), nil
	case "opencode":
		h := mustHome(env)
		cfg, skills, share := env.OCConfig, env.OCSkills, env.OCShare
		if cfg == "" {
			cfg = filepath.Join(h, ".config", "opencode")
		}
		if skills == "" {
			skills = filepath.Join(h, ".opencode")
		}
		if share == "" {
			share = filepath.Join(h, ".local", "share", "opencode")
		}
		return agent.OpenCodeHomes(cfg, skills, share), nil
	default:
		return nil, fmt.Errorf("unknown agent %s", agentName)
	}
}

func mustHome(env Env) string {
	if env.UserHome != "" {
		return env.UserHome
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func snapDir(env Env) (string, error) {
	if env.SnapDir != "" {
		return env.SnapDir, nil
	}
	h := mustHome(env)
	if h == "" {
		return "", fmt.Errorf("home directory unknown")
	}
	return filepath.Join(h, ".local", "share", "pindah", "snapshots"), nil
}

func insideHome(out string, homes map[string]string) (bool, string) {
	out = filepath.Clean(out)
	ids := make([]string, 0, len(homes))
	for id := range homes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		home := filepath.Clean(homes[id])
		if home == "" || home == "." {
			continue
		}
		rel, err := filepath.Rel(home, out)
		if err != nil {
			continue
		}
		if rel == "." || (!strings.HasPrefix(rel, "..") && rel != out) {
			return true, home
		}
	}
	return false, ""
}

func usage(env Env, msg string) Code {
	fmt.Fprintln(env.Stderr, msg)
	return CodeUsage
}

func ioerr(env Env, err error) Code {
	fmt.Fprintln(env.Stderr, err.Error())
	return CodeIO
}

func classifyWrite(env Env, err error) Code {
	if os.IsNotExist(err) || os.IsPermission(err) {
		return ioerr(env, err)
	}
	// A collector already turned missing files into skips. Remaining
	// write failures that are path errors are usage (bad archive shape);
	// disk failures are IO.
	msg := err.Error()
	if strings.Contains(msg, "nothing to pack") {
		return usage(env, msg)
	}
	return ioerr(env, err)
}
