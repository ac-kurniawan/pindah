# Archive

Format version is `1`.

The file is a gzip-compressed tar. Members use forward slashes and no leading slash. The first member is `manifest.json`. Every other member is a payload file.

```text
manifest.json
payload/<home-id>/<path-relative-to-that-home>
```

`home-id` values:

| home-id | omp | opencode |
| --- | --- | --- |
| `omp` | `~/.omp` | |
| `opencode-config` | | `~/.config/opencode` |
| `opencode-skills` | | `~/.opencode` |
| `opencode-share` | | `~/.local/share/opencode` |

A missing source file is skipped. It is not an error. A missing source directory that the allowlist names is skipped. An empty allowlist (nothing existed to pack) is an error: `backup` exits non-zero and does not leave a partial archive.

## manifest.json

```json
{
  "format": 1,
  "agent": "omp",
  "created": "2026-09-26T16:30:00Z",
  "files": [
    {
      "path": "payload/omp/mcp.json",
      "sha256": "<hex>",
      "size": 828,
      "mode": 420
    }
  ]
}
```

- `format` is the integer `1`.
- `agent` is `omp` or `opencode`.
- `created` is UTC RFC3339.
- `files` is sorted by `path`.
- `sha256` is the hex digest of the uncompressed file bytes.
- `size` is the uncompressed byte length.
- `mode` is the permission bits only (`os.FileMode.Perm()`), as a JSON number. Owner, uid, and timestamps are not stored.

`list` prints `format`, `agent`, `created`, then one line per file: mode, size, sha256, path. It does not extract.

## Integrity

`restore` and `list` reject the archive when:

- `manifest.json` is missing, duplicated, or not valid JSON of this shape
- `format` is not `1`
- `agent` is not a v1 agent
- a `files` entry path does not start with `payload/<home-id>/` for a home that agent owns
- a `files` entry path escapes its home (`..`, absolute, or a link)
- a tar member is not `manifest.json` and not listed in `files`
- a listed file is missing from the tar
- gzip or tar is truncated
- a payload member's sha256 or size does not match the manifest
- a tar member is a directory, symlink, hardlink, or device

Symlinks are never packed and never extracted. `backup` skips them and records nothing for them.

## Write rules

`backup` writes the archive to a temporary file in the destination directory, fsyncs, then renames it into place. A failed backup leaves no archive at the final path.

`restore` checks every destination path before it writes any file. On collision without `--force`, the homes are unchanged. With `--force`, it still refuses if it cannot replace a path (directory where a file should be, or the reverse) and writes nothing in that case.
