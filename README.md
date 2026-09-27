# pindah

Go CLI that snapshots and restores coding-agent config. v1 agents: omp, opencode.

```text
pindah backup omp
pindah backup opencode
pindah list ~/.local/share/pindah/snapshots/pindah-omp-20260926T163000Z.tar.gz
pindah restore ~/.local/share/pindah/snapshots/pindah-omp-20260926T163000Z.tar.gz
pindah restore <archive> --force
```

`backup` writes `~/.local/share/pindah/snapshots/pindah-<agent>-<UTC>.tar.gz` unless `-o` is set. The archive holds `manifest.json` plus the allowlisted files. Sessions, sqlite, logs, caches, plugin caches, and `node_modules` stay out.

`restore` reads the agent from the manifest. If a destination file already exists, it exits and writes nothing. `--force` replaces only files that are in the archive.

Credential fields are copied as bytes. The archive is not encrypted. `backup` says so on stderr.

Specs: [doc/specs](doc/specs/00-overview.md).

## Not in v1

Migrate (omp to opencode, or the reverse). Claude and Codex. Repo-level `AGENTS.md`. Re-downloading plugins after restore.
