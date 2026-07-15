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

Two additional sources -- **Smithery** and **open-web search (SerpAPI, via Google search
results)** -- are gated behind a real ForceDream billing key (`FD_LIVE_KEY`, the same
`fd_live_...` credential already used for `invoke`), a positive prepaid balance, and a
paid-search entitlement. Each successful search charges the account, atomically and only
after a real result comes back -- see `BillingSafety.md` for the full mechanism, including
why a client-side quota alone can never protect a shared key. Without `FD_LIVE_KEY` set (or
without a sufficient balance), the command still works using the three free sources above,
clearly labeling these two as requiring sign-in rather than silently omitting them.

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
| `FD_LIVE_KEY` | `invoke`, Smithery/web search | Real `fd_live_...` billing key; needs a positive balance for the two paid search sources, which also charge it per search |
| `GITHUB_TOKEN` | optional | Raises GitHub search's rate limit (not a paid API) |

No API key is ever baked into the binary. The real Smithery/SerpAPI keys live only on the
ForceDream backend, never in this CLI or its distributed binaries. Missing or rejected
credentials mean that specific source is skipped, clearly labeled with the real reason
(sign-in required, insufficient balance, feature not enabled) -- the rest of the command
still works.

## Links

- MCP server: https://github.com/forcedreamai/forcedream-mcp
- Go SDK (used internally by this CLI): https://github.com/forcedreamai/forcedream-sdk-go
- Official MCP Registry: https://registry.modelcontextprotocol.io

## License

MIT
