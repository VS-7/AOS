// Package build holds every user-visible identifier of the product and the
// values stamped into the binary at link time.
//
// The product name is a placeholder (see ADR-0000). Renaming the product means
// editing this file and the module path — nothing else. No other package may
// hardcode the product name, the state directory or the error prefix.
package build

// Brand constants. Every user-visible identifier of the product lives here.
const (
	Name        = "aos"  // binary name, config dir suffix, MCP server name
	DisplayName = "AOS"  // window title, CLI header
	ErrorPrefix = "AOS"  // error code prefix: AOS_MEMORY_NOT_FOUND
	StateDir    = ".aos" // ~/.aos and <workspace>/.aos
	EnvPrefix   = "AOS"  // AOS_SERVER_PORT, AOS_TOKEN
	Port        = 5326
)
