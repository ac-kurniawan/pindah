# CLI

Module path: `github.com/arcaku-labs/pindah`. Binary name: `pindah`.

Exit codes:

| code | when |
| --- | --- |
| 0 | success |
| 1 | bad args, unknown agent, archive rejected, destination collision, output path inside a packed tree, empty allowlist |
| 2 | IO error reading a home or writing an archive |

Usage errors print one line to stderr and exit 1. No stack traces.

## backup

```text
pindah backup <agent> [-o path]
```

`<agent>` is `omp` or `opencode`.

Default `-o` is `~/.local/share/pindah/snapshots/pindah-<agent>-<UTC>.tar.gz`. The snapshots directory is created if missing, mode `0700`. The archive file mode is `0600`.

Before packing, if the output path (after cleaning) is inside any home that this backup reads, exit 1.

Stdout, on success, is the archive path and nothing else. Stderr gets the credential warning.

## restore

```text
pindah restore <archive> [--force]
```

Read the manifest, verify the archive (see `01-archive.md`), then stat every destination path.

Destination path is the home root for that member's home-id, plus the relative path after `payload/<home-id>/`.

Without `--force`, if any destination exists, print the colliding paths to stderr, write nothing, exit 1.

With `--force`, replace those files. Create missing parent directories at mode `0700`. File mode comes from the manifest. Do not delete local files that the archive does not contain.

A restore never creates a home that the archive does not mention. It does create a mentioned home if that home directory is missing.

## list

```text
pindah list <archive>
```

Verify the archive the same way `restore` does, then print the manifest. Write nothing outside stdout.

## Flags not in v1

No `--dry-run`. No global config file. No `migrate`.
