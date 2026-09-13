package build

// The flavours a binary is built in, as Flavour reports them.
//
// A flavour is compiled in, never stamped: `-tags webui` is what embeds the
// web interface in aosd (cmd/aosd/webui_embed.go), and the same tag is what
// picks Flavour's definition, so a binary cannot say one thing and carry the
// other.
const (
	// FlavourStandard is every binary built without the tag: the daemon
	// aos-desktop supervises, the terminal, the window — and the raw aosd a
	// release publishes for the updater.
	FlavourStandard = "standard"
	// FlavourServer is the daemon with the web interface compiled in, which
	// the server tarball and `task build:server` produce. The release feed
	// publishes no aosd of this flavour, so the updater must not swap the
	// standard one in over it: the server would come back answering the API
	// only, with no interface to open in a browser.
	FlavourServer = "server"
)
