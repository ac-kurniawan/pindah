# pindah

Go CLI. v1 is snapshot and restore of omp and opencode config. Specs in `doc/specs/` are the contract. If this file and a spec disagree, the spec wins.

## Layout

One module, `github.com/arcaku-labs/pindah`. Standard library plus `archive/tar` and `compress/gzip`. No CLI framework.

```text
cmd/pindah/main.go
internal/archive/archive.go
internal/agent/omp.go
internal/agent/opencode.go
internal/cli/cli.go
```

`internal/agent` returns the list of files to pack for one agent. It does not know about tar. `internal/archive` writes and reads the tar.gz from that list. `internal/cli` parses args and sets exit codes from `doc/specs/04-cli.md`.

## Rules

- Allowlists are the lists in `02-omp.md` and `03-opencode.md`. Do not pack "everything except junk".
- Do not merge the two omp MCP files.
- Do not follow `skills.paths` outside the homes those specs name.
- Do not redact secrets. Do not add encryption.
- Do not add `migrate`, `--dry-run`, claude, or codex.
- Skip missing source files. Fail if the allowlist packs zero files.
- Skip symlinks and any path named `.git`. Never store a symlink member.
- `restore` without `--force` is all-or-nothing: stat every destination first, write none on collision.
- Archive mode is `0600`. Snapshots directory mode is `0700`.

## Verify

```text
go test ./...
```

Tests cover: manifest round-trip, sha256 mismatch rejected, collision without `--force` writes nothing, `--force` replaces only archived paths, symlink and `.git` skipped, output path inside a home refused.
