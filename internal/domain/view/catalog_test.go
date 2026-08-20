package view_test

import (
	"testing"

	"github.com/OWNER/aos/internal/domain/view"
)

// The catalog is the contract between the Go domain and the React design
// system. The original keeps its registry in the frontend and the backend does
// not know it, which is what lets an agent reference a component that does not
// exist; here the catalog is generated and a drift fails the gate.
func TestTheCatalogCarriesTheDesignSystem(t *testing.T) {
	specs := view.Catalog()
	if len(specs) < 50 {
		t.Fatalf("the catalog has %d components, which is too few to be the real one", len(specs))
	}

	for _, name := range []string{"Card", "Stack", "Table", "Button", "SplitPageLayout"} {
		spec, ok := view.LookupComponent(name)
		if !ok {
			t.Fatalf("%s is missing from the catalog", name)
		}
		if spec.Description == "" {
			t.Fatalf("%s has no description; the agent reads it to choose", name)
		}
	}
}

// A component that accepts children is what a tree can nest under. Getting this
// wrong means a valid view is refused, or an invalid one is written.
func TestAContainerAcceptsChildrenAndALeafDoesNot(t *testing.T) {
	stack, ok := view.LookupComponent("Stack")
	if !ok {
		t.Fatal("Stack is missing")
	}
	if !stack.AcceptsChildren {
		t.Fatal("Stack does not accept children, but it is the layout primitive")
	}
}

func TestAnUnknownComponentIsNotFound(t *testing.T) {
	if _, ok := view.LookupComponent("ThisWasNeverBuilt"); ok {
		t.Fatal("the catalog answered for a component that does not exist")
	}
}

// The catalog is sorted so that a regeneration with no change produces no diff.
func TestTheCatalogIsSorted(t *testing.T) {
	specs := view.Catalog()
	for i := 1; i < len(specs); i++ {
		if specs[i-1].Name > specs[i].Name {
			t.Fatalf("%q comes after %q", specs[i-1].Name, specs[i].Name)
		}
	}
}
