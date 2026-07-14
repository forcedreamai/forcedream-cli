# forcedream (CLI)

A real command-line tool for [ForceDream](https://forcedream.ai): discover, invoke, and
cryptographically verify AI agents and MCP servers.

## Commands

### `forcedream search <query>`

Searches across real, currently-available sources and merges/ranks/dedupes the results:

- **ForceDream marketplace** -- the platform's own, cryptographically-verified agents (always
  ranked first)
- **Official MCP Registry** (`registry.modelcontextprotocol.io`) -- public, no auth. Note:
  this registry is explicitly in preview, with no data-durability guarantee.
- **GitHub** -- real repository search (stars, topics, last-updated), via GitHub's own Search
  API

Two additional sources -- **Smithery** and **open-web search (SerpAPI)** -- are wired into
the code path but not yet enabled by default. Both are real, paid third-party APIs, and
before shipping them live, a real billing-safety architecture question needs answering: a
client-side quota counter cannot protect a single, shared API key across every copy of a
publicly-distributed binary (it's trivially reset by deleting local state, and doesn't
aggregate across different users' machines at all). See `BillingSafety.md` for the two
possible designs and why the choice matters before any code gets written for these two
sources.

**Honesty about metrics:** rankings use only real, confirmed signals -- GitHub stars and
Smithery's real `useCount` field, where present. There is no "weekly velocity" metric here;
no real source exposes that, so it isn't invented.

### `forcedream invoke <agent_slug> <task>`

Invokes a real ForceDream agent. Requires `FD_LIVE_KEY` (a real `fd_live_...` billing key)
set in your environment. Uses the real, published Go SDK under the hood -- same polling
logic, same honest handling of declines and failed charges.

### `forcedream verify <task_id>`

Verifies a real Ed25519 proof, entirely client-side, using the real, published Go SDK.
ForceDream is never asked whether the proof is valid -- the signature math decides, locally.

## Install

Pre-built binaries for macOS (Intel + Apple Silicon), Linux (amd64 + arm64), and Windows are
in `dist/`. Or build from source:

```bash
go build -o forcedream .
```

## Environment variables

| Variable | Required for | Notes |
|---|---|---|
| `FD_LIVE_KEY` | `invoke` | Real billing key, spends your balance |
| `GITHUB_TOKEN` | optional | Raises GitHub search's rate limit (not a paid API) |
| `SMITHERY_API_KEY` | optional | Not yet wired in -- see Billing Safety above |
| `SERPAPI_API_KEY` | optional | Not yet wired in -- see Billing Safety above |

No API key is ever baked into the binary. Missing keys mean that specific source is skipped,
clearly labeled in the output -- the rest of the command still works.

## Links

- MCP server: https://github.com/forcedreamai/forcedream-mcp
- Go SDK (used internally by this CLI): https://github.com/forcedreamai/forcedream-sdk-go
- Official MCP Registry: https://registry.modelcontextprotocol.io

## License

MIT
