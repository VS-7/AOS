package skillfetch

import "github.com/OWNER/aos/internal/core/apperr"

func errRefNotSupported(ref string) error {
	return apperr.New("SKILLFETCH_REF_NOT_SUPPORTED").
		Causer("skillfetch.Local.Fetch").
		Msgf("a local directory has no ref to select; got %q", ref).
		Issue("ref", ref).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "fetch without a ref, or from a source that has versions"})
}

func errNotAPackage(source string) error {
	return apperr.New("SKILLFETCH_NOT_A_PACKAGE").
		Causer("skillfetch.Local.Fetch").
		Msgf("%q is not a skill package: no readable SKILL.md", source).
		Issue("source", source).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "point at a directory containing a SKILL.md"})
}

func errManifest(source string, cause error) error {
	return apperr.New("SKILLFETCH_MANIFEST_INVALID").
		Causer("skillfetch.Local.Fetch").
		Msgf("%q's SKILL.md front matter does not parse: %v", source, cause).
		Issue("source", source).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "fix the YAML front matter of SKILL.md"})
}

func errDecode(path string, cause error) error {
	return apperr.New("SKILLFETCH_DECODE_FAILED").
		Causer("skillfetch.Local.Fetch").
		Msgf("%q does not decode: %v", path, cause).
		Issue("path", path).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "fix the file's contents"})
}

func errRead(path string, cause error) error {
	return apperr.New("SKILLFETCH_READ_FAILED").
		Causer("skillfetch.Local.Fetch").
		Msgf("could not read %q: %v", path, cause).
		Issue("path", path).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "check that the package directory is readable"})
}

// errOutsidePackage fires when a resource the manifest declares resolves
// outside the package directory — the read this whole file exists to refuse.
func errOutsidePackage(uri string, cause error) error {
	return apperr.New("SKILLFETCH_RESOURCE_OUTSIDE_PACKAGE").
		Causer("skillfetch.Local.Fetch").
		Msgf("resource %q resolves outside the package", uri).
		Issue("uri", uri).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "reference only files inside the skill package"})
}
