package gateway

import (
	"context"

	"github.com/OWNER/aos/internal/core/build"
	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "gateway",
	Tool:    "Gateway",
	Summary: "Start, stop and inspect the background daemon.",
	Doc: `Supervise the ` + build.Name + ` daemon.

This is the one group that runs locally instead of through the daemon, for the
obvious reason: asking a process that is not running to start itself does not
work.

The state is read by crossing the record on disk with whether the process it
names is actually alive, so a daemon that crashed reads as stale rather than as
running — and starting again cleans it up instead of refusing.

## When to use
- To bring the daemon up before anything that needs it
- To find out whether it is running, and whether it is actually answering

## When NOT to use
- Not to restart after every change: the configuration file is re-read while
  the daemon runs`,
	Hint: "Running and answering are different states. A daemon can be alive and not serving — status reports both.",
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[StatusInput, State]{
		Group:   "gateway",
		Name:    "status",
		Summary: "Report whether the daemon is running.",
		Doc: `Report the state of the daemon: stopped, stale, or running.

Stale means there is a record of a daemon and the process it names is gone —
which is what a crash looks like from here. Starting again clears it.

Running is also reported with whether the daemon answered its health check.
A process can be alive and not serving, and the two are worth telling apart.`,
		Examples: []command.Example{
			{Description: "is it up", Input: StatusInput{}},
		},
		Local:       true,
		Registry:    true,
		Annotations: command.Annotations{Title: "Gateway status", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Status,
	})

	command.MustRegister(reg, command.Command[StartInput, State]{
		Group:   "gateway",
		Name:    "start",
		Summary: "Start the daemon.",
		Doc: `Start the daemon, if it is not already running.

Calling this when it is running returns the existing state rather than failing:
the caller asked for a daemon to be up, and it is.

Success means the daemon answered its health endpoint, not merely that a process
was spawned. A daemon that started and could not bind its port is a failure, and
this reports it as one.`,
		Examples: []command.Example{
			{Description: "bring it up", Input: StartInput{}},
		},
		Local:       true,
		Registry:    true,
		Annotations: command.Annotations{Title: "Start the daemon", IdempotentHint: true},
		Handler:     svc.Start,
	})

	command.MustRegister(reg, command.Command[StopInput, StopOutput]{
		Group:   "gateway",
		Name:    "stop",
		Summary: "Stop the daemon.",
		Doc: `Ask the daemon to shut down, and insist if it does not.

It is asked politely first and killed after the shutdown timeout. The result
says which happened: a daemon that had to be killed is a daemon that lost
whatever it had not yet written, and that is worth knowing.`,
		Local:       true,
		Registry:    true,
		Annotations: command.Annotations{Title: "Stop the daemon", DestructiveHint: true, IdempotentHint: true},
		Handler:     svc.Stop,
	})

	command.MustRegister(reg, command.Command[RestartInput, State]{
		Group:   "gateway",
		Name:    "restart",
		Summary: "Stop the daemon and start it again.",
		Doc: `Restart the daemon.

If you are an agent reading this: you are running inside the process you are
about to restart, and this call will not return to you. That is not a bug and it
is not avoidable — the connection carrying your request ends with the process
serving it. Say what you are doing before you call it.`,
		Local:       true,
		Registry:    true,
		Annotations: command.Annotations{Title: "Restart the daemon", DestructiveHint: true},
		Handler:     svc.Restart,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, StatusInput) (State, error)    = (*Service)(nil).Status
	_ func(context.Context, StopInput) (StopOutput, error) = (*Service)(nil).Stop
	_ func(context.Context, RestartInput) (State, error)   = (*Service)(nil).Restart
)
