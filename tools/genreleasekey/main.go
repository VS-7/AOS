// Command genreleasekey generates the Ed25519 keypair
// internal/domain/update's release-signature verification is built on.
//
// Run it once per key rotation, never as part of a build: the public half
// gets committed (internal/app's embedded release-pubkey.pub); the private
// half is what a release process signs checksums.txt with (relsig.Sign) and
// must never be committed, logged, or shipped in a binary.
package main

import (
	"fmt"
	"os"

	"github.com/OWNER/aos/internal/core/relsig"
)

func main() {
	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		fmt.Fprintln(os.Stderr, "genreleasekey:", err)
		os.Exit(1)
	}
	fmt.Println("# public key — commit this as internal/app/release-pubkey.pub")
	fmt.Println(pub)
	fmt.Println()
	fmt.Println("# private key — keep this OUT of the repository (a CI secret, a password manager)")
	fmt.Println(priv)
}
