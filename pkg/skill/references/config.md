# Config

Read and change the installation configuration.

Read and change the global configuration of the installation, stored at
`~/.aos/config.json`.

## What an agent can and cannot do
Secrets are never returned to an agent: provider keys, the API token, the
session secret and the tunnel token come back as a fingerprint, never in full.
An agent may change region and general preferences; anything under `security`,
any provider key and the tunnel token are refused with a call to action pointing
the human at the app.

## Commands

### `config_get`

Read the configuration, with secrets redacted.

Read the whole configuration.

Every secret is replaced by a fingerprint — enough to tell which key is
configured, useless to whoever reads it. `reveal` is honoured only for a human
on an interactive terminal; an agent that asks for it is refused.

- read the configuration

### `config_update`

Change configuration fields by dotted path.

Change one or more configuration fields.

Fields are addressed by dotted path, the same path the file shows:
`{"region.timezone": "America/Sao_Paulo"}"`. An agent may only write the
agent-writable allowlist; anything else is refused rather than silently ignored.

- set the timezone

