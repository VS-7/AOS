package app_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/OWNER/aos/internal/testx"
	"github.com/OWNER/aos/internal/transport/mcpserver"
)

// TestGroupDocumentationGolden pins the text a model reads before choosing a
// tool. There is no natural assertion for "the description is good"; what can
// be asserted is that it changed, and that the change was reviewed.
func TestGroupDocumentationGolden(t *testing.T) {
	a := newApp(t)
	for _, group := range a.Registry.Groups() {
		t.Run(group.Name, func(t *testing.T) {
			var b strings.Builder
			fmt.Fprintf(&b, "# tool: %s\n\n", group.Tool)
			b.WriteString(mcpserver.DescriptionOf(group))
			b.WriteString("\n\n## actions\n\n")
			for _, d := range group.Commands {
				fmt.Fprintf(&b, "### %s\n\n%s\n\n%s\n\n", d.Key(), d.Summary(), d.Doc())
				schema, err := json.MarshalIndent(d.InputSchema(), "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				fmt.Fprintf(&b, "```json\n%s\n```\n\n", schema)
			}
			testx.AssertString(t, "docs/"+group.Name, b.String())
		})
	}
}
