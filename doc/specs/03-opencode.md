# opencode

`backup opencode` reads three homes. Each has its own archive home-id. A path that does not exist is skipped.

| home-id | root |
| --- | --- |
| `opencode-config` | `~/.config/opencode` |
| `opencode-skills` | `~/.opencode` |
| `opencode-share` | `~/.local/share/opencode` |

## opencode-config

Files:

```text
AGENTS.md
opencode.jsonc
ocx.jsonc
config.json
package.json
package-lock.json
plugins/*.ts
plugins/*.js
profiles/**/*
```

`profiles/**/*` is every regular file under `profiles/`, at any depth.

`skills/` is packed recursively, except any path whose name is `.git`, and except symlinks. Same rule as omp skills. A skill git clone is the working tree without `.git`.

Not packed from this home: `node_modules/`, `.gitignore`, and anything else not named above.

`opencode.jsonc` `skills.paths` may point outside this home (on this machine, a tilde path at a karpathy skill tree). Those outside paths are not followed. Only trees named in this spec are packed.

## opencode-skills

Pack `skills/` recursively, except `.git` paths and symlinks. Do not pack `bin/`, `node_modules/`, `package.json`, or anything else in `~/.opencode`.

The copies under `~/.opencode/skills` are not referenced from `opencode.jsonc` `skills.paths`. They are still packed, because the v1 decision was to take every skill dir, not only referenced ones.

## opencode-share

Pack this file only:

```text
auth.json
```

Not packed: `opencode.db` and its wal/shm, `log/`, `snapshot/`, `worktree/`, `plugins/`, `repos/`, `tool-output/`, `workspace/`, `delegations/`.

## Secrets

`opencode.jsonc` and `auth.json` are copied as bytes, including API keys. `backup` prints the same stderr warning as omp. Restore writes them back unchanged. There is no encryption.
