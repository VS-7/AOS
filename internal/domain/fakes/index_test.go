package fakes_test

import (
	"testing"

	"github.com/OWNER/aos/internal/core/search"
	"github.com/OWNER/aos/internal/domain/fakes"
	"github.com/OWNER/aos/internal/domain/testsuite"
)

// TestIndexSatisfiesTheContract is what makes the memory tests that run with an
// index mean something: the persistent adapter runs the same suite.
func TestIndexSatisfiesTheContract(t *testing.T) {
	testsuite.RunIndexContract(t, testsuite.IndexContract{
		New: func(*testing.T) search.Index { return fakes.NewIndex() },
	})
}
