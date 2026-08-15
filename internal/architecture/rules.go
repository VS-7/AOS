// Package architecture holds the structural rules of the project and the tests
// that enforce them.
//
// An architectural convention without verification becomes a documented
// exception within three months. These rules cost eighty lines and make the
// violation a build failure instead of a code review argument.
package architecture

// Module is the import path of this module. It is a literal on purpose: the
// rules must keep working when the product is renamed, and ADR-0000 makes
// "github.com/OWNER/aos" greppable for exactly that operation.
const Module = "github.com/OWNER/aos"

// Forbidden maps a guarded package prefix to the import prefixes it may not
// reach, directly or in any file under it.
var Forbidden = map[string][]string{
	Module + "/internal/domain": {
		"net/http", "os/exec", "database/sql",
		"github.com/go-chi/", "github.com/spf13/",
		"github.com/wailsapp/", "github.com/modelcontextprotocol/",
		"github.com/coder/websocket",
		"modernc.org/sqlite", "github.com/blevesearch/",
		"github.com/robfig/cron/",
		Module + "/internal/transport",
		Module + "/internal/adapters",
		Module + "/internal/runtime",
	},
	// The arrow points inwards: the domain knows the Command Layer, the
	// Command Layer does not know the domain. A new command therefore never
	// requires touching core/command.
	Module + "/internal/core/command": {
		Module + "/internal/domain",
	},
	// pkg/ is the part of the tree that could be useful outside this project.
	Module + "/pkg": {
		Module + "/internal",
	},
}

// ClientBinaries may not link domain code: the CLI and the desktop app are
// clients of the daemon. The gateway is the documented exception — it
// supervises an OS process and runs locally by design.
var ClientBinaries = []string{
	Module + "/cmd/aos",
	Module + "/cmd/aos-desktop",
}

// DomainExceptions are the domain packages a client binary may link.
var DomainExceptions = []string{
	Module + "/internal/domain/gateway",
}

// NonFeatureDomainDirs are the directories under internal/domain that are not
// features: the executable port contracts and the fakes every domain test runs
// on. They are here rather than in the domain because they must not be shipped
// in a binary, and internal/domain is what a domain test can import without
// breaking the dependency rule.
var NonFeatureDomainDirs = map[string]bool{
	"testsuite": true,
	"fakes":     true,
}

// RequiredFeatureFiles are the files every domain feature must have.
//
// commands.go joins this list in phase 2, when the Command Layer exists to be
// imported; declaring it earlier would fail every feature for a file that
// cannot yet be written.
var RequiredFeatureFiles = []string{
	"entity.go",
	"service.go",
	"port.go",
}
