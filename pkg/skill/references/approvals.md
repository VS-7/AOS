# Approvals

See and answer the tool calls waiting on a person.

Tool calls an agent is waiting on.

A policy hook can answer "ask" instead of allow or deny, and when it does the
call stops here until somebody decides. Deciding is not something an agent can
do: these commands are deliberately outside the agent's own tool registry.

A request that nobody answers becomes a denial when its deadline passes. There
is no path by which waiting turns into approval.

## Commands

### `approvals_decide`

Allow or refuse a waiting tool call.

Answer a waiting tool call.

The payload may be corrected before approving, which is often what somebody
actually wants: not "no", but "not like that". Approving once does not approve
the same call in every future context.

### `approvals_list`

List the tool calls waiting for a decision.

List the tool calls waiting for a person to allow or refuse them, oldest first.

