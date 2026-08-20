package view_test

import (
	"context"
	"testing"

	"github.com/OWNER/aos/internal/domain/view"
)

// Components is the introspection an agent calls before composing a screen —
// reading the catalog through the service, the same path Task 7's
// screen-composition code will use, rather than the package function directly.
func TestComponentsServesTheGeneratedCatalog(t *testing.T) {
	svc := view.NewService(view.Deps{})

	specs, err := svc.Components(context.Background())
	if err != nil {
		t.Fatalf("Components: %v", err)
	}
	if len(specs) != len(view.Catalog()) {
		t.Fatalf("Components returned %d components, Catalog() has %d", len(specs), len(view.Catalog()))
	}

	found := false
	for _, spec := range specs {
		if spec.Name == "Card" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Components did not include Card")
	}
}
