# Models

Which models the connected providers actually serve.

Ask each connected provider what it can serve, right now.

The answer comes from the provider, not from a list inside this build. A model
that appears here exists and this installation can authenticate to it; a model
that does not appear either is not served or is not reachable with the
credential configured.

## Commands
- **list** — every connected provider's catalogue, or one named provider's

## When to use
- **Before pointing an agent or a slot at a model:** so the id is one that exists
- **When a turn fails with an unknown model:** to see what the provider replaced it with

## When NOT to use
- Not to find out which providers are *connected* — that is the configuration

## Commands

### `models_list`

List the models the connected providers serve.

Ask every connected provider for its catalogue, or one named provider for its own.

Each provider is asked over the network with the credential this installation
has for it, so this is a real question with a real latency, not a lookup.

- everything reachable
- one provider

