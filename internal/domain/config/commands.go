package config

import (
	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before touching the configuration.
var GroupDoc = command.GroupDoc{
	Name:    "config",
	Tool:    "Config",
	Summary: "Read and change the installation configuration.",
	Doc: `Read and change the global configuration of the installation, stored at
` + "`~/" + build.StateDir + `/config.json` + "`" + `.

## What an agent can and cannot do
Secrets are never returned to an agent: provider keys, the API token, the
session secret and the tunnel token come back as a fingerprint, never in full.
An agent may change region and general preferences; anything under ` + "`security`" + `,
any provider key and the tunnel token are refused with a call to action pointing
the human at the app.`,
	Hint: "Reading the configuration is safe. Changing it affects every workspace of this installation.",
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[GetInput, Config]{
		Group:   "config",
		Name:    "get",
		Summary: "Read the configuration, with secrets redacted.",
		Doc: `Read the whole configuration.

Every secret is replaced by a fingerprint — enough to tell which key is
configured, useless to whoever reads it. ` + "`reveal`" + ` is honoured only for a human
on an interactive terminal; an agent that asks for it is refused.`,
		Examples: []command.Example{
			{Description: "read the configuration", Input: GetInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read the configuration", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[UpdateInput, Config]{
		Group:   "config",
		Name:    "update",
		Summary: "Change configuration fields by dotted path.",
		Doc: `Change one or more configuration fields.

Fields are addressed by dotted path, the same path the file shows:
` + "`{\"region.timezone\": \"America/Sao_Paulo\"}\"`" + `. An agent may only write the
agent-writable allowlist; anything else is refused rather than silently ignored.`,
		Examples: []command.Example{
			{
				Description: "set the timezone",
				Input:       UpdateInput{Set: map[string]any{"region.timezone": "America/Sao_Paulo"}},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update the configuration", IdempotentHint: true},
		Handler:     svc.Update,
	})
}
