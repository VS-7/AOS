# Marketplace

A remote registry of installable skills — discover and install from repositories.

A skill's own domain (skills_install) installs from a local package already
on disk. marketplace is the layer above it: a registry of packages that are
not on disk yet, configured once (Git-based by default, HTTP for a hosted
index) and searched or fetched by "<owner>/<repo>".

## Commands
- **discovery** — search every configured registry
- **get** — read one listing's full manifest before installing it
- **install** — fetch and install, going through the same verify/consent
  path skills_install does

## When to use
- **Before installing something you have not seen configured locally:**
  discovery first — a listing's Permissions are visible before any bytes are
  fetched, per ADR-0015

## When NOT to use
- Not for a package already on disk — that is skills_install directly

## Commands

### `marketplace_discovery`

Search every configured marketplace registry.

Search across every configured registry, merging results. A registry that does not answer is skipped, not fatal.

- search by free text
- search by owner

### `marketplace_get`

Read one listing's full manifest.

Read a listing in full — its manifest and the permissions it declares — before installing it.

- read a listing before installing it

### `marketplace_install`

Fetch and install a skill from a registry.

Fetches "<owner>/<repo>" from the named registry, or every configured
registry until one answers, then verifies and installs it exactly as
skills_install does — including asking for consent unless the caller
already has it (ADR-0007).

- install from any configured registry

