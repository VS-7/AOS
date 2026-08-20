package skill

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
)

// errManifestExceeded is why ADR-0015 exists: content that exceeds what a
// package declared is refused, and the refusal names the excess — never
// trimmed and installed anyway, and never installed on the strength of the
// manifest's word alone.
func errManifestExceeded(excess []string) error {
	return apperr.New("SKILL_MANIFEST_EXCEEDED").
		Causer("skill.ManifestVerifier.VerifyManifest").
		Msgf("the package contains more than its manifest declares: %s", strings.Join(excess, ", ")).
		Issue("excess", excess).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "declare every collection, agent, exec binary and network host the package actually uses in its permissions, or remove what is undeclared",
		})
}

// errInstallNotApproved fires when a human — or a caller acting for one —
// declines, or when the approval channel times out. A timeout is a denial,
// never an approval (event.Broker's own invariant), so this is the one path
// out of a missing "yes".
func errInstallNotApproved(source, reason string) error {
	return apperr.New("SKILL_INSTALL_NOT_APPROVED").
		Causer("skill.Installer.Install").
		Msgf("installing %q was not approved: %s", source, reason).
		Issue("source", source).
		Issue("reason", reason).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{
			Label: "a human must approve a skill install before anything is written; retry once approved",
		})
}

// errCollectionNameTaken is the controller ruling: an install that brings a
// collection whose name is already registered is refused, naming the
// collision, rather than letting collections.Registry.Register silently
// replace whatever is there — see Installer.Install's own comment.
func errCollectionNameTaken(name string) error {
	return apperr.New("SKILL_COLLECTION_NAME_TAKEN").
		Causer("skill.Installer.Install").
		Msgf("a collection named %q is already registered", name).
		Issue("collection", name).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{
			Label: "rename the collection in the package's manifest, or remove the existing one first",
		})
}

// errUpdateFailed wraps a failure writing an update to an already-installed
// skill — turning Active on or off, the one field this domain lets change in
// place.
func errUpdateFailed(id string, cause error) error {
	return apperr.New("SKILL_UPDATE_FAILED").
		Causer("skill.Installer.Update").
		Msgf("updating %q failed: %v", id, cause).
		Issue("id", id).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}

func errNotFound(id string) error {
	return apperr.New("SKILL_NOT_FOUND").
		Causer("skill.Installer.Get").
		Msgf("no skill %q is installed", id).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "list installed skills before reading or uninstalling one by id"})
}

func errSourceRequired() error {
	return apperr.New("SKILL_SOURCE_REQUIRED").
		Causer("skill.Installer.Install").
		Msgf("a skill install needs a source").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "name a local directory or a registry address to install from"})
}

// errManifestInvalid fires when the manifest itself is not usable — no name
// to derive an id from, or a name with characters that are not safe as a
// directory segment — before verification or consent is ever reached.
func errManifestInvalid(reason string) error {
	return apperr.New("SKILL_MANIFEST_INVALID").
		Causer("skill.Installer.Install").
		Msgf("the manifest is not usable: %s", reason).
		Issue("reason", reason).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "give the skill a name usable as a directory: lowercase letters, digits, hyphen and underscore",
		})
}

// errApplyFailed wraps a failure partway through applying a verified,
// consented package. The skill's own record is written last precisely so
// this can happen without leaving a half-registered skill — see
// Installer.Install.
func errApplyFailed(id, step string, cause error) error {
	return apperr.New("SKILL_APPLY_FAILED").
		Causer("skill.Installer.Install").
		Msgf("installing %q failed while applying %s: %v", id, step, cause).
		Issue("id", id).
		Issue("step", step).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "the skill's directory may hold partial content; delete it and retry the install"})
}

// errUninstallFailed wraps a failure partway through Uninstall. It fires
// before the skill's files are removed — hooks, toolsets, collections and
// views are torn down first — so the skill is left fully registered rather
// than half torn down.
func errUninstallFailed(id, step string, cause error) error {
	return apperr.New("SKILL_UNINSTALL_FAILED").
		Causer("skill.Installer.Uninstall").
		Msgf("uninstalling %q failed while %s: %v", id, step, cause).
		Issue("id", id).
		Issue("step", step).
		Status(apperr.StatusInternalServerError).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "retry; if it persists, this is a bug"})
}
