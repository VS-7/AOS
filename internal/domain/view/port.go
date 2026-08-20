package view

import (
	"context"
	"encoding/json"
)

// Commands is the slice of the command registry an Action needs: whether a
// command exists, and how to run one. Task 7 adds the Collections port beside
// it, once the collection domain exists to be referenced.
type Commands interface {
	Has(name string) bool
	Invoke(ctx context.Context, name string, in json.RawMessage) (json.RawMessage, error)
}
