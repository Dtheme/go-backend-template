English | [简体中文](README.zh-CN.md)

# go-backend-template

A minimal Go HTTP service (standard library only) that ships with a spec-driven workflow for coding agents. Claude Code and Codex read one rule file; `make check` machine-checks that the docs still describe the code.

## Quick start

```bash
make run     # :8080
make check   # go vet + unit tests + gofmt + spec-check
```

Fork it into a real project — rewrites the module path, rebuilds symlinks, verifies the copy builds and tests clean:

```bash
bash .ai/skills/init-project/rename_module.sh my-service github.com/me/my-service
```

Agents run the same thing as a skill: `/init-project` in Claude Code, `$init-project` in Codex. To retrofit an existing Go repo instead, use `spec-coding-init`.

## Commands

| Command | Runs |
| --- | --- |
| `make check` | `go vet`, unit tests, `gofmt -l`, `spec-check` |
| `make test-integration` | tests behind `//go:build integration` |
| `make test-race` | `go test -race ./...` |
| `make smoke` | builds the binary, boots it, curls every route |
| `make spec-init VERSION=x.y.z` | generates a version's technical plan; refuses if the human requirement is missing |
| `make lint` / `make vuln` | golangci-lint / govulncheck, skipped when not installed |

`spec-check` (`cmd/spec-check`) enforces ten rules across the docs: required files and headings, the four symlinks, version directory naming, every `F-x.y.z-NNN` in a requirement appearing in its plan, the delivery-status block, and the Postman collection's shape.

## How a version gets built

`Specs/requirements/` is human-owned and read-only to agents. `Specs/technical/` is agent-owned and must track the code.

1. Read the architecture doc, the version requirement, and the protocol conventions.
2. Ask about every ambiguity. Guessing is not allowed.
3. Freeze the API contract: method, path, request, response, error table.
4. `make spec-init VERSION=x.y.z`, fill the plan, **get the user's confirmation**, set `stage: implementing`.
5. Failing test first, then the smallest implementation that passes it.
6. Run the four gates plus the `api-verify` skill, then **wait for the user to accept**.
7. Write real results back into the plan, architecture doc, Postman collection and smoke script.

Two steps stop for a human. Step 4 needs the plan confirmed before any code is written; step 6 needs acceptance before the version can be called done. `spec-check` rejects `stage: delivered` while `user_acceptance` is still `pending`, so the stop is enforced by a command rather than by discipline.

## What ships with it

| Mechanism | Where |
| --- | --- |
| Rules, single source | `.ai/ai-rules.md`, symlinked as `CLAUDE.md` and `AGENTS.md` |
| Spec docs | `Specs/requirements/` (human), `Specs/technical/` (agent) |
| Gates | `Makefile`, `cmd/spec-check`, `scripts/smoke.sh` |
| Skills | `.ai/skills/`: `init-project`, `spec-coding-init`, `api-verify`, `sync-ai-assets`, `spec-graph-workflow`, plus vendored [`go-development`](https://github.com/netresearch/go-development-skill) |
| Subagents | `.claude/agents/`: `spec-reviewer` (read-only), `spec-implementer` |
| Hook | `PostToolUse` → `scripts/check-format.sh`, gofmt feedback after every edit |
| MCP | none required; Postman MCP is optional and never gates anything |

### Vendored skill

`.ai/skills/go-development/` is not written here. It is copied unmodified from [netresearch/go-development-skill](https://github.com/netresearch/go-development-skill), pinned at version 1.15.1, and covers Go practice this template does not restate: test layering, common `-race` traps, `slog`, linting, fuzzing, dependency upgrades.

`SOURCE.md` in that directory is the citation record: upstream URL, pinned version, SHA-256 for all 26 files so anyone can re-verify nothing drifted, the licence split, and the five points where upstream advice contradicts `.ai/ai-rules.md`. On every one of those five, the local rules win. Upgrading means replacing the whole directory and rewriting `SOURCE.md`; never patch it in place, or the checksums stop meaning anything.

## Demo version

`1.0.0` implements two endpoints over in-memory storage so the workflow has something real to run against:

```
POST /v1/notes       201 {"id","title","content","created_at"}
GET  /v1/notes/{id}  200, or 404 {"error":{"code":"not_found","message":"note not found"}}
```

Its requirement, plan, tests, six review findings and state graph are all committed. Delete `Specs/*/1.0.0/`, `internal/note/`, `internal/memstore/` and `internal/httpapi/notes*.go` when you start your own.

## Optional: spec-graph

`cmd/spec-graph` records which evidence belongs to which code candidate and refuses to advance a version when a gate is missing or an input changed under it. Six stages, seven events, every transition guarded. See `docs/spec-graph/`.

## Scope

This is an MVP. The rules, skills, subagents and hook cover a generic Go HTTP service and nothing else. Real work needs more: a skill for your deploy flow, MCP servers for your database and internal APIs, gates for your CI, review rules for your domain. What is here is a starting shape you extend, not a finished toolchain.

## Docs

18 articles walking through every mechanism, in Chinese: [docs/guide/](docs/guide/), or read them online on [Feishu](https://ncnhrkchbfwf.feishu.cn/wiki/SVHxwROhfiYBE2k2I35cJ0IUnYe).

## License

MIT, except `.ai/skills/go-development/`. That directory is vendored from [netresearch/go-development-skill](https://github.com/netresearch/go-development-skill): MIT for the code and scripts, CC-BY-SA-4.0 for the 20 reference documents, copyright Netresearch DTT GmbH. Modify those documents and they stay under CC-BY-SA-4.0 with attribution. The pinned version and per-file checksums live in its `SOURCE.md`. Delete the directory if you would rather not carry a share-alike obligation.
