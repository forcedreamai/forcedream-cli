# Billing Safety: Smithery and SerpAPI integration

**Status: implemented (Model B). This document is now a record of the decision, not an open question.**

## The real problem, and the decision made

The original spec asked for a client-side daily quota counter inside the CLI binary --
e.g., "stop calling SerpAPI after 50 calls/day" -- enforced in the distributed binary
itself.

This does not actually protect a shared API key. Concretely:

- The CLI is distributed publicly (Homebrew, direct download, static binaries for five
  platforms). Anyone can download and run it.
- A local quota counter (a state file on disk, e.g. `~/.forcedream/quota.json`) is reset to
  zero by deleting that file, or by running the binary in a fresh environment/container.
- Every different machine running the CLI starts its own separate, unlinked counter. There
  is no way for client-side code to know how many calls *anyone else* has already made
  today against the same shared key.

**Model B (shared, ForceDream-paid key, server-side enforcement) was chosen.** The CLI does
not call Smithery or SerpAPI directly at all -- it calls two real ForceDream backend
proxies (`/v1/search/smithery-proxy`, `/v1/search/web-proxy`) that hold the real keys
server-side only (Vercel env vars, never sent to any client) and enforce a real, global
quota in Redis (daily for Smithery, monthly for SerpAPI, matching each provider's real
plan). This is the only design that actually protects a shared key: the counter lives in
one place that can't be reset by deleting local state.

## Access control (added after the initial proxy build)

Both proxies are additionally gated behind a real ForceDream billing key: a valid
`fd_live_...` key (via `FD_LIVE_KEY` -- the same credential the CLI already uses for
`invoke`; an earlier version of this used a separate `FD_ACCOUNT_KEY`/`sk_fd_...` pair,
which turned out to check the wrong ledger entirely -- see the note below), a positive
prepaid balance (the real, established fd_live_ billing substrate, `liveCharge`/
`liveBalance`, the same one agent invocation already uses -- not the separate earnings
ledger), and a `paid-search` entitlement flag
(currently defaulted to enabled for all real accounts, since no subscription-tier system
exists yet to differentiate who should/shouldn't have it -- the field is real and checkable
for when that system exists). The CLI detects and reports three specific rejection reasons
(`auth_required`, `insufficient_funds`, `feature_not_enabled`) rather than a generic
failure.

**Correction, made before this was ever charged against in practice:** the first version of
this gate checked `fd:earn1:user:*:balance_pence` (an `sk_fd_`-keyed EARNINGS ledger --
money a developer earns when others use *their* agents), not the actual prepaid,
*spendable* balance that pays for using a platform service. Confirmed directly against the
real source: `liveCharge()`, already used for agent invocation and other platform-tool
charges, operates on a completely separate, `fd_live_`-keyed ledger. Rebuilt to use that
real substrate throughout -- one consequence being no second credential is needed at all;
the CLI's existing `FD_LIVE_KEY` now covers `invoke` and both search sources.

## Metering (implemented)

Each successful search charges the account: an atomic, idempotent Lua-script decrement
(`liveCharge`, confirmed live via a real, temporary diagnostic route before being relied
on -- Upstash's client genuinely supports `EVAL`), applied only *after* a real, successful
external result -- never on a failed external call. If the atomic charge still fails even
though an earlier, non-atomic balance check passed (a genuinely rare race: balance moved in
the real-world seconds a slow external call takes), the output is withheld and a 402
returned -- external cost already sunk, but never given away for free, matching an existing
precedent already in this exact backend file rather than inventing a new tradeoff.

Revenue is routed through the existing, audited `recordRealRevenue('platform_tool', ...)`
pipeline -- the same category other platform-tool charges already use -- rather than a new,
parallel revenue stream that wouldn't show up in the same dashboards.

The real per-search price (`SEARCH_PRICE_PENCE` in the backend) is currently a visible,
explicit 1p placeholder. That number is a genuine business decision, not something decided
by this document or invented unilaterally -- adjust it before relying on real revenue
figures from this feature.

