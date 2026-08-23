// Package relsig verifies the signature over a release's checksums file
// before internal/domain/update ever installs anything.
//
// docs/08 - Entrega/Auto-Update.md calls for "a minisign signature ... using
// a public key embedded in the binary". No mature Go minisign library exists
// in this module's dependency graph, and hand-reimplementing minisign's own
// wire format — its trusted-comment line, the second signature computed over
// the first signature plus that comment — from memory risks getting it
// subtly wrong while looking right, which is worse for a verification gate
// than not claiming byte-compatibility at all. Same call this codebase
// already made for TOON and for SQLite in pure Go (see the design's own
// risk table): an own implementation, disclosed, built on the primitive
// minisign itself is built on — Ed25519 — rather than a guessed
// reimplementation of a format this package was never checked against the
// real minisign CLI for.
//
// What internal/domain/update actually needs — a signature over bytes,
// checked against an embedded public key, mandatory, no way to disable —
// is exactly what this package gives it.
package relsig

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
)

// ErrInvalidSignature covers every way verification can fail: a malformed
// key, a malformed signature, or a well-formed signature that does not
// verify. The caller has exactly one thing to do with any of them — refuse
// to install — so there is one error to check for, not three.
var ErrInvalidSignature = errors.New("relsig: signature does not verify against the public key")

// GenerateKey creates a new Ed25519 keypair, base64-encoded for storage as
// text: the public half as an embedded file
// (internal/domain/update's //go:embed release-pubkey.pub), the private
// half as a release-signing secret that must never ship in a binary or a
// repository — see cmd/tools's key-generation script, not this package,
// for where the private half is meant to live.
func GenerateKey() (publicKeyB64, privateKeyB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), nil
}

// Sign signs message with a base64-encoded private key from GenerateKey.
func Sign(privateKeyB64 string, message []byte) (signatureB64 string, err error) {
	priv, err := decodePrivate(privateKeyB64)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, message)), nil
}

// Verify checks signatureB64 over message against a base64-encoded public
// key from GenerateKey. Every failure mode collapses to ErrInvalidSignature
// — see the package doc comment.
func Verify(publicKeyB64 string, message []byte, signatureB64 string) error {
	pub, err := decodePublic(publicKeyB64)
	if err != nil {
		return ErrInvalidSignature
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureB64))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	if !ed25519.Verify(pub, message, sig) {
		return ErrInvalidSignature
	}
	return nil
}

func decodePublic(s string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("relsig: malformed public key")
	}
	return ed25519.PublicKey(raw), nil
}

func decodePrivate(s string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, errors.New("relsig: malformed private key")
	}
	return ed25519.PrivateKey(raw), nil
}
