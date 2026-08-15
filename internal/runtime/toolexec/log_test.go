package toolexec_test

import (
	"io"
	"log/slog"
)

// discardLog keeps the warning about a failed spill out of the test output,
// where it would look like a failure rather than the behaviour under test.
func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
