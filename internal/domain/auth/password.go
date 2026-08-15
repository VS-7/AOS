package auth

import (
	"bufio"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// MinPasswordLen is twelve, not the original's six.
//
// The account being protected can run shell commands on the machine. Six
// characters is a policy for a forum login, and the original applies it to a
// credential that authorises code execution.
const MinPasswordLen = 12

// The argon2id parameters, kept from the original because they are sound:
// 64 MiB of memory, two passes, one lane.
const (
	argonTime    uint32 = 2
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

//go:embed breached.txt
var breachedRaw string

// breached is the set of passwords that must never be accepted, loaded once.
//
// The list is embedded rather than fetched: a password check that makes a
// network call fails offline, leaks the password's prefix to a third party, and
// turns an authentication step into a dependency on someone else's uptime.
var breached = sync.OnceValue(func() map[string]bool {
	set := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(breachedRaw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[strings.ToLower(line)] = true
	}
	return set
})

// ValidatePassword enforces the policy: long enough, and not one of the
// passwords that are tried first.
func ValidatePassword(p string) error {
	if len([]rune(p)) < MinPasswordLen {
		return errPasswordTooShort(len([]rune(p)))
	}
	if breached()[strings.ToLower(strings.TrimSpace(p))] {
		return errPasswordBreached()
	}
	return nil
}

// BreachedCount reports how many passwords the embedded list holds. It exists
// so a test can fail when the list is emptied by accident — a policy that
// silently stops rejecting anything is worse than no policy, because it reads
// as protection.
func BreachedCount() int { return len(breached()) }

// HashPassword derives an argon2id hash with a fresh random salt, encoded in
// the standard PHC string format so the parameters travel with the hash.
func HashPassword(p string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	return encodeHash(salt, deriveKey(p, salt)), nil
}

// VerifyPassword checks a password against a stored hash in constant time.
//
// It derives with the parameters the stored hash carries rather than the
// current constants, so raising the cost later does not lock everyone out of
// the hashes written before the change.
func VerifyPassword(stored, candidate string) bool {
	h, ok := decodeHash(stored)
	if !ok {
		return false
	}
	got := argon2.IDKey([]byte(candidate), h.salt, h.time, h.memory, h.threads, uint32(len(h.key)))
	return subtle.ConstantTimeCompare(got, h.key) == 1
}

func deriveKey(p string, salt []byte) []byte {
	return argon2.IDKey([]byte(p), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
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
