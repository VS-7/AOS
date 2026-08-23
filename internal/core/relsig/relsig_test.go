package relsig_test

import (
	"errors"
	"testing"

	"github.com/OWNER/aos/internal/core/relsig"
)

func TestSignAndVerifyRoundTrip(t *testing.T) {
	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("aos-checksums-v0.10.0\nsha256 abc123 aos_darwin_arm64\n")

	sig, err := relsig.Sign(priv, message)
	if err != nil {
		t.Fatal(err)
	}
	if err := relsig.Verify(pub, message, sig); err != nil {
		t.Fatalf("a signature just produced by Sign did not verify: %v", err)
	}
}

func TestVerifyRejectsATamperedMessage(t *testing.T) {
	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sig, err := relsig.Sign(priv, []byte("original checksums"))
	if err != nil {
		t.Fatal(err)
	}

	if err := relsig.Verify(pub, []byte("tampered checksums"), sig); !errors.Is(err, relsig.ErrInvalidSignature) {
		t.Fatalf("got %v, want ErrInvalidSignature", err)
	}
}

func TestVerifyRejectsAWrongKey(t *testing.T) {
	_, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	otherPub, _, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	message := []byte("release payload")
	sig, err := relsig.Sign(priv, message)
	if err != nil {
		t.Fatal(err)
	}

	if err := relsig.Verify(otherPub, message, sig); !errors.Is(err, relsig.ErrInvalidSignature) {
		t.Fatalf("got %v, want ErrInvalidSignature", err)
	}
}

func TestVerifyRejectsGarbageSignature(t *testing.T) {
	pub, _, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := relsig.Verify(pub, []byte("payload"), "not-base64!!"); !errors.Is(err, relsig.ErrInvalidSignature) {
		t.Fatalf("got %v, want ErrInvalidSignature", err)
	}
	if err := relsig.Verify(pub, []byte("payload"), ""); !errors.Is(err, relsig.ErrInvalidSignature) {
		t.Fatalf("empty signature: got %v, want ErrInvalidSignature", err)
	}
}

func TestVerifyRejectsMalformedPublicKey(t *testing.T) {
	_, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sig, err := relsig.Sign(priv, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if err := relsig.Verify("not-a-key", []byte("payload"), sig); !errors.Is(err, relsig.ErrInvalidSignature) {
		t.Fatalf("got %v, want ErrInvalidSignature", err)
	}
}

func TestSignRejectsMalformedPrivateKey(t *testing.T) {
	if _, err := relsig.Sign("not-a-key", []byte("payload")); err == nil {
		t.Fatal("expected an error signing with a malformed private key")
	}
}

// GenerateKey's output round-trips through whitespace a text file (or a
// human pasting it) might add — trailing newline included.
func TestVerifyToleratesSurroundingWhitespace(t *testing.T) {
	pub, priv, err := relsig.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sig, err := relsig.Sign(priv, []byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if err := relsig.Verify("  "+pub+"\n", []byte("payload"), "\n"+sig+"  "); err != nil {
		t.Fatalf("whitespace-padded key/signature should still verify: %v", err)
	}
}
