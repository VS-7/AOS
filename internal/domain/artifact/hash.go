package artifact

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// The argon2id parameters — kept identical to internal/domain/auth's own,
// since both are hashing a secret against offline brute force and there is
// no reason for the two to disagree. Not imported from there directly: a
// domain package does not import another domain package (see
// internal/architecture's dependency rule) — the ~30 lines below are
// duplicated rather than coupling artifact's password hashing to auth's.
const (
	argonTime    uint32 = 2
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// Argon2Hasher is the default PasswordHasher: argon2id with a fresh random
// salt per hash, encoded in the standard PHC string format so the parameters
// travel with the hash and a later cost increase does not invalidate what is
// already stored.
type Argon2Hasher struct{}

func (Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return encodeHash(salt, key), nil
}

func (Argon2Hasher) Verify(password, stored string) (bool, error) {
	h, ok := decodeHash(stored)
	if !ok {
		return false, nil
	}
	got := argon2.IDKey([]byte(password), h.salt, h.time, h.memory, h.threads, uint32(len(h.key)))
	return subtle.ConstantTimeCompare(got, h.key) == 1, nil
}

func encodeHash(salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

// phc is a parsed argon2id hash in the standard string format.
type phc struct {
	memory  uint32
	time    uint32
	threads uint8
	salt    []byte
	key     []byte
}

func decodeHash(stored string) (phc, bool) {
	parts := strings.Split(stored, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return phc{}, false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return phc{}, false
	}
	var out phc
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &out.memory, &out.time, &out.threads); err != nil {
		return phc{}, false
	}
	if out.memory == 0 || out.time == 0 || out.threads == 0 {
		return phc{}, false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return phc{}, false
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return phc{}, false
	}
	out.salt, out.key = salt, key
	return out, true
}
