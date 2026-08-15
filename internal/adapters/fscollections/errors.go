package fscollections

import "github.com/OWNER/aos/internal/core/collections"

// The engine owns the error vocabulary of the Repository port, so that a second
// adapter cannot invent codes of its own and break every caller that branches
// on behaviour. These are aliases, not new errors.
var (
	errNotFound      = collections.NotFoundError
	errAlreadyExists = collections.AlreadyExistsError
	errConflict      = collections.ConflictError
	errIO            = collections.IOError
	errOutside       = collections.OutsideRootError
	errNotOwned      = collections.NotOwnedError
)
