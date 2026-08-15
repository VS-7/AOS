package command

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errInvalidInput(key string, err error) *apperr.Error {
	return apperr.New("COMMAND_INVALID_INPUT").
		Causer(key).
		Msgf("the payload of %s could not be decoded: %v", key, err).
		Issue("tool", key).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "inspect the input schema for this action instead of retrying blindly",
			Tool:  key,
			Input: map[string]any{"schema": true},
		}).
		Wrap(err)
}

func errValidation(key string) *apperr.Error {
	return apperr.New("COMMAND_VALIDATION_FAILED").
		Causer(key).
		Msgf("the payload of %s is not valid", key).
		Issue("tool", key).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "read the issues, inspect the contract with schema:true, then fix the payload",
			Tool:  key,
			Input: map[string]any{"schema": true},
		})
}

func errUnknownCommand(key string, known []string) error {
	return apperr.New("COMMAND_NOT_FOUND").
		Causer("command.Registry.Lookup").
		Msgf("%q is not a command", key).
		Issue("tool", key).
		Issue("didYouMean", strings.Join(nearest(key, known, 3), ", ")).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the available commands",
			Command: build.Name + " --help",
		})
}

func errUnknownAction(tool, action string, known []string) error {
	return apperr.New("COMMAND_ACTION_UNKNOWN").
		Causer(tool).
		Msgf("%q has no action %q", tool, action).
		Issue("tool", tool).
		Issue("action", action).
		Issue("actions", strings.Join(known, ", ")).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "call the tool with schema:true to see every action and its input",
			Tool:  tool,
			Input: map[string]any{"schema": true},
		})
}

// nearest returns the known keys closest to a mistyped one, by edit distance.
// The original answers a mistyped command with "Did you mean …"; so do we.
func nearest(want string, known []string, limit int) []string {
	type scored struct {
		key  string
		dist int
	}
	var all []scored
	for _, k := range known {
		d := distance(want, k)
		if d <= len(want)/2+2 {
			all = append(all, scored{k, d})
		}
	}
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].dist < all[j-1].dist; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	out := make([]string, 0, limit)
	for i := 0; i < len(all) && i < limit; i++ {
		out = append(out, all[i].key)
	}
	return out
}

func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
