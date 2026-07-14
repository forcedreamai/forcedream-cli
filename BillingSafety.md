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

Both proxies are additionally gated behind a real ForceDream account: a valid `sk_fd_...`
key (via `FD_ACCOUNT_KEY` -- a different credential from `FD_LIVE_KEY`, which is used for
agent invocation), a positive account balance, and a `paid-search` entitlement flag
(currently defaulted to enabled for all real accounts, since no subscription-tier system
exists yet to differentiate who should/shouldn't have it -- the field is real and checkable
for when that system exists). The CLI detects and reports three specific rejection reasons
(`auth_required`, `insufficient_funds`, `feature_not_enabled`) rather than a generic
failure.

**Known, deliberate gap:** this checks that balance is positive but does not deduct
anything per search. A user with any positive balance passes indefinitely; nothing meters
search usage into real, ongoing revenue. Flagged directly to the person who requested this
rather than silently decided either way -- whether metering should be added is a separate
design question about whether this feature should fund a real SerpAPI/Smithery plan
upgrade, not something resolved by this document.

