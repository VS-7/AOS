package collections

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

func errPatternUnterminated(raw string, at int) error {
	return apperr.New("COLLECTION_PATTERN_INVALID").
		Causer("collections.Compile").
		Msgf("unterminated placeholder in pattern %q at offset %d", raw, at).
		Issue("pattern", raw).
		Status(apperr.StatusInternalServerError)
}

func errPatternBadField(raw, name string) error {
	return apperr.New("COLLECTION_PATTERN_INVALID").
		Causer("collections.Compile").
		Msgf("invalid placeholder %q in pattern %q", name, raw).
		Issue("pattern", raw).
		Status(apperr.StatusInternalServerError)
}

func errPatternDuplicateField(raw, name string) error {
	return apperr.New("COLLECTION_PATTERN_INVALID").
		Causer("collections.Compile").
		Msgf("placeholder %q appears twice in pattern %q", name, raw).
		Issue("pattern", raw).
		Status(apperr.StatusInternalServerError)
}

func errPatternInvalid(raw string, err error) error {
	return apperr.New("COLLECTION_PATTERN_INVALID").
		Causer("collections.Compile").
		Msgf("pattern %q does not compile", raw).
		Issue("pattern", raw).
		Status(apperr.StatusInternalServerError).
		Wrap(err)
}

func errPatternNotWritable(raw string) error {
	return apperr.New("COLLECTION_PATTERN_NOT_WRITABLE").
		Causer("collections.Pattern.Build").
		Msgf("pattern %q contains a wildcard and cannot build a path", raw).
		Issue("pattern", raw).
		Status(apperr.StatusInternalServerError)
}

// errPatternMissingField is the error that closes an entire class of bug:
// writing a record to a partially resolved path.
func errPatternMissingField(raw, field string, k Key) error {
	return apperr.New("COLLECTION_KEY_INCOMPLETE").
		Causer("collections.Pattern.Build").
		Msgf("cannot build a path from %q: placeholder %q has no value", raw, field).
		Issue("pattern", raw).
		Issue("missing", field).
		Issue("given", k.String()).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "provide every placeholder of the pattern: " + strings.Join(fieldsOf(raw), ", "),
		})
}

func errPatternFieldHasSeparator(raw, field, value string) error {
	return apperr.New("COLLECTION_KEY_INVALID").
		Causer("collections.Pattern.Build").
		Msgf("placeholder %q must not contain a path separator", field).
		Issue("pattern", raw).
		Issue("field", field).
		Issue("value", value).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "use an identifier without '/' — a record key names one path element",
		})
}

func fieldsOf(raw string) []string {
	p, err := Compile(raw)
	if err != nil {
		return nil
	}
	return p.Fields()
}

func errNoWritablePattern(collection string) error {
	return apperr.New("COLLECTION_NOT_WRITABLE").
		Causer("collections.Model.WritePattern").
		Msgf("collection %q declares no pattern that can build a path", collection).
		Issue("collection", collection).
		Status(apperr.StatusInternalServerError)
}

// NotFoundError, AlreadyExistsError, ConflictError, IOError, OutsideRootError
// and NotOwnedError are exported because every adapter of the Repository port
// must report the same conditions with the same codes. A second adapter that
// invented its own codes would break the contract suite and every caller that
// branches on behaviour.
func NotFoundError(collection string, k Key) error {
	return apperr.New("COLLECTION_NOT_FOUND").
		Causer("collections.Repo.Get").
		Msgf("no record %s in collection %q", k.String(), collection).
		Issue("collection", collection).
		Issue("record", k.String()).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the collection to find the right key",
			Command: build.Name + " " + collection + " list",
		})
}

func AlreadyExistsError(collection string, k Key) error {
	return apperr.New("COLLECTION_ALREADY_EXISTS").
		Causer("collections.Repo.Create").
		Msgf("record %s already exists in collection %q", k.String(), collection).
		Issue("collection", collection).
		Issue("record", k.String()).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "update the existing record instead of creating it again",
		})
}

// errConflict is the optimistic-concurrency failure. Without it, two parallel
// instances of the same agent editing one memory lose an edit in silence.
func ConflictError(collection, path string, expect, actual Version) error {
	return apperr.New("COLLECTION_CONFLICT").
		Causer("collections.Repo.Update").
		Msgf("%q changed since it was read", path).
		Issue("collection", collection).
		Issue("path", path).
		Issue("expectedSize", expect.Size).
		Issue("actualSize", actual.Size).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "reload the record and reapply the change — another writer got there first",
		})
}

func errDecode(collection, path string, err error) error {
	return apperr.New("COLLECTION_DECODE_FAILED").
		Causer("collections.Decode").
		Msgf("%q is not a valid %s record", path, collection).
		Issue("collection", collection).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		CTA(apperr.CallToAction{
			Label: "fix the YAML front matter of the file, or delete it and let the agent recreate it",
		}).
		Wrap(err)
}

func errEncode(collection string, err error) error {
	return apperr.New("COLLECTION_ENCODE_FAILED").
		Causer("collections.Encode").
		Msgf("a %s record cannot be serialised", collection).
		Issue("collection", collection).
		Status(apperr.StatusInternalServerError).
		Wrap(err)
}

func IOError(op, path string, err error) error {
	return apperr.New("COLLECTION_IO_FAILED").
		Causer("collections.Repo."+op).
		Msgf("%s failed on %q", op, path).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(err)
}

func OutsideRootError(path, root string) error {
	return apperr.New("COLLECTION_PATH_ESCAPES_ROOT").
		Causer("collections.Repo.resolve").
		Msgf("%q resolves outside the workspace root", path).
		Issue("path", path).
		Issue("root", root).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "a record key must name one path element; it cannot traverse directories",
		})
}

func errUnknownCollection(name string) error {
	return apperr.New("COLLECTION_UNKNOWN").
		Causer("collections.ModelOf").
		Msgf("no collection named %q", name).
		Issue("collection", name).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label: "use one of the native collections: " + strings.Join(nativeNames(), ", "),
		})
}

func nativeNames() []string {
	out := make([]string, 0, len(natives))
	for _, d := range natives {
		out = append(out, d.Name)
	}
	return out
}

// NotOwnedError reports a file that does not belong to the collection asked to
// read it — the sign of a pattern that changed without a migration.
func NotOwnedError(collection, path string) error {
	return apperr.New("COLLECTION_PATH_NOT_OWNED").
		Causer("collections.Repo.readAt").
		Msgf("%q does not match any pattern of collection %q", path, collection).
		Issue("collection", collection).
		Issue("path", path).
		Status(apperr.StatusInternalServerError)
}
