# omp

Home root: `~/.omp`. Archive home-id: `omp`.

`backup omp` packs only the paths below, relative to the home root. A path that does not exist is skipped.

## Files

```text
AGENTS.md
mcp.json
marketplaces.json
models.yaml
agent/AGENTS.md
agent/config.yml
agent/mcp.json
agent/extensions/*.ts
plugins/installed_plugins.json
plugins/omp-plugins.lock.json
plugins/package.json
plugins/bun.lock
```

`agent/extensions/*.ts` is the directory's `*.ts` files, not a recursive walk. Subdirectories are not packed.

## Skills

Pack `skills/` recursively, except:

- any path whose name is `.git`
- symlinks

That includes empty skill directories' files only. Directories themselves are not tar members. A skill that is a git clone is packed as the working tree without `.git`.

## Not packed

Anything not in the lists above. In particular:

- `agent/*.db`, `agent/*.db-wal`, `agent/*.db-shm`
- `agent/sessions/`, `agent/blobs/`, `agent/terminal-sessions/`
- `agent/.env`, `agent/last-changelog-version`
- `plugins/cache/`, `plugins/node_modules/`, `plugins/package-lock.json` if present later
- `logs/`, `cache/`, `context-mode/`, `run/`, `jev/`, `puppeteer/`, `graft/`
- `autoqa.db`, `gpu_cache.json`, `install-id`

## Two MCP files

`mcp.json` and `agent/mcp.json` are both packed when both exist. They are not merged. On this machine they differ: the root file has a `notion` server the agent file lacks, and the agent file has `atlassian` and `context-mode` servers the root file lacks. `context-mode` stores an absolute path. Restore writes that path back unchanged.

Plugin records (`installed_plugins.json`, `omp-plugins.lock.json`, `marketplaces.json`) are packed. The plugin cache is not. Restore does not run a package manager.

## Secrets

Credential fields inside packed files are copied as bytes. `backup` prints one warning to stderr: the archive contains credential material. It does not name the keys.
