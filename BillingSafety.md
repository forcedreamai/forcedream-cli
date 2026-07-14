# Billing Safety: Smithery and SerpAPI integration

**Status: not yet implemented. Read this before wiring either source in.**

## The real problem

The original spec asked for a client-side daily quota counter inside the CLI binary --
e.g., "stop calling SerpAPI after 50 calls/day" -- enforced in the distributed binary
itself.

This does not actually protect a shared API key. Concretely:

- The CLI is distributed publicly (Homebrew, direct download, static binaries for three
  platforms). Anyone can download and run it.
- A local quota counter (a state file on disk, e.g. `~/.forcedream/quota.json`) is reset to
  zero by deleting that file, or by running the binary in a fresh environment/container.
- Every different machine running the CLI starts its own separate, unlinked counter. There
  is no way for client-side code to know how many calls *anyone else* has already made
  today against the same shared key.

If a single Smithery/SerpAPI API key (paid for by ForceDream, e.g. one already added to
Vercel) is the key this CLI actually uses, a client-side cap provides **no real protection**
against runaway billing across many users -- only against one honest user's own accidental
overuse on their own machine.

## Two coherent designs -- pick one before implementing

### A. Bring-your-own-key (BYOK)

Each end user sets their own `SMITHERY_API_KEY` / `SERPAPI_API_KEY`, using their own
account, billed to them. In this model:

- A local, per-machine quota tracker is a genuine, useful *courtesy* feature -- it stops one
  user from blowing through their own daily budget by accident.
- This matches how essentially every CLI wrapping a paid third-party API works (`aws`,
  `gh`, etc.) and is the natural reading of "read from environment variables, never bake
  into the binary."
- **This is what the current code (`internal/discovery`) is structured for.** No further
  architectural work needed -- just wire in the Smithery/SerpAPI clients and the local quota
  tracker as originally specced.

### B. Shared, ForceDream-paid key

If the intent is for the CLI to make these paid calls against ForceDream's own account,
regardless of who's running it, the only design that actually protects that spend is
**server-side enforcement**:

- The CLI does not call Smithery/SerpAPI directly at all.
- Instead, it calls a real, new ForceDream backend endpoint (e.g. `/v1/search/web-proxy`,
  `/v1/search/smithery-proxy`) that holds the real keys server-side only (Vercel env vars,
  never sent to any client) and enforces the real, global daily quota in Redis -- the same
  pattern every other billing/quota mechanism in ForceDream already uses.
- This requires new backend work (two new proxy endpoints, quota tracking keys in Redis)
  before the CLI side can be finished.

## Do not build a client-side-only "hard cap" for a shared key

Doing so would look like a safety feature while providing none of the protection the
original request asked for ("we cannot be scammed out of freemium"). If B is the intent,
say so and the backend work gets scoped properly; if A is the intent, this is already ready
to finish.
