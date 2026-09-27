# Overview

`pindah` is a Go CLI that snapshots and restores coding-agent config. v1 agents are `omp` and `opencode`. v1 does not migrate one agent into another.

A snapshot is one `.tar.gz`. `manifest.json` sits at the archive root. Payload paths are relative to a named home, so two homes never collide.

Default archive path:

```text
~/.local/share/pindah/snapshots/pindah-<agent>-<UTC>.tar.gz
```

UTC format is `20060102T150405Z`. `-o` overrides the path. If `-o` is inside a tree being packed, `backup` exits non-zero and writes nothing.

## Commands

```text
pindah backup <agent> [-o path]
pindah restore <archive> [--force]
pindah list <archive>
```

`restore` reads the agent from the manifest. There is no agent flag on `restore`.

`backup` copies credential fields as bytes. It does not redact and it does not encrypt. It prints that the archive contains credential material.

`restore` writes each payload file back to its home. If any destination path already exists, it exits non-zero and writes nothing, unless `--force`. `--force` replaces only paths present in the archive. Local files that are not in the archive stay.

## Out of v1

- `migrate`
- claude, codex, and any other agent
- project instruction files (`AGENTS.md` inside a repo)
- session transcripts, sqlite databases, logs, caches
- plugin and npm caches, including `node_modules`
- re-downloading plugins after restore
- encryption
