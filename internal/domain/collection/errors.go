package collection

import (
	"strings"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/core/build"
)

// The four declarative hook actions this build understands, listed once for
// the error that names them.
var hookActions = []string{
	string(ActionSetTimestamp), string(ActionSlugify),
	string(ActionDefaultTo), string(ActionComputeFrom),
}

// The seven field types this engine understands, listed once for the error
// that names them.
var fieldTypes = []string{
	string(TypeString), string(TypeNumber), string(TypeBoolean),
	string(TypeDate), string(TypeEnum), string(TypeRef), string(TypeList),
}

func errFieldRequired(id, field string) error {
	return apperr.New("COLLECTION_FIELD_REQUIRED").
		Causer("collection.Validate").
		Msgf("%q is required by the collection %q and was not given", field, id).
		Issue("collection", id).
		Issue("field", field).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "include the field, or read the collection's schema to see what it declares"})
}

func errFieldWrongType(id, field, want string, got any) error {
	return apperr.New("COLLECTION_FIELD_WRONG_TYPE").
		Causer("collection.Validate").
		Msgf("%q expects %s and was given %T", field, want, got).
		Issue("collection", id).
		Issue("field", field).
		Issue("expected", want).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "send " + field + " as " + want})
}

func errFieldNotInEnum(id, field, got string, allowed []string) error {
	return apperr.New("COLLECTION_FIELD_NOT_IN_ENUM").
		Causer("collection.Validate").
		Msgf("%q is not one of the values %q declares for %q", got, id, field).
		Issue("collection", id).
		Issue("field", field).
		Issue("given", got).
		Issue("allowed", allowed).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use one of the values in the issue \"allowed\": " + strings.Join(allowed, ", ")})
}

// errFieldNotUnique is a conflict, not a bad request: the record itself is
// well-formed, and what makes it unacceptable is a fact about what else is
// already stored, which the caller could not have known without asking.
func errFieldNotUnique(id, field string, value any) error {
	return apperr.New("COLLECTION_FIELD_NOT_UNIQUE").
		Causer("collection.Validate").
		Msgf("another record of %q already has %q = %v", id, field, value).
		Issue("collection", id).
		Issue("field", field).
		Issue("value", value).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: "search the collection for that value before creating a new record"})
}

func errFieldUndeclared(id, field string) error {
	return apperr.New("COLLECTION_FIELD_UNDECLARED").
		Causer("collection.Validate").
		Msgf("%q declares no field named %q", id, field).
		Issue("collection", id).
		Issue("field", field).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "declare the field on the collection, or remove it from the record"})
}

func errFieldUnnamed(id string) error {
	return apperr.New("COLLECTION_FIELD_UNNAMED").
		Causer("collection.ValidateSchema").
		Msgf("%q declares a field with no name", id).
		Issue("collection", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "give every field a name; it is how a record refers to it"})
}

func errFieldDuplicated(id, field string) error {
	return apperr.New("COLLECTION_FIELD_DUPLICATED").
		Causer("collection.ValidateSchema").
		Msgf("%q declares %q twice", id, field).
		Issue("collection", id).
		Issue("field", field).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "remove the duplicate; the second declaration would hide the first"})
}

func errFieldTypeUnknown(id, field, typ string) error {
	return apperr.New("COLLECTION_FIELD_TYPE_UNKNOWN").
		Causer("collection.ValidateSchema").
		Msgf("%q is not a type this engine understands for field %q", typ, field).
		Issue("collection", id).
		Issue("field", field).
		Issue("given", typ).
		Issue("allowed", fieldTypes).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use one of the seven types in the issue \"allowed\": " + strings.Join(fieldTypes, ", ")})
}

func errEnumEmpty(id, field string) error {
	return apperr.New("COLLECTION_ENUM_EMPTY").
		Causer("collection.ValidateSchema").
		Msgf("%q declares %q as an enum with no values", id, field).
		Issue("collection", id).
		Issue("field", field).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "declare the values an enum field accepts; one with none accepts nothing"})
}

func errRefMissing(id, field string) error {
	return apperr.New("COLLECTION_REF_MISSING").
		Causer("collection.ValidateSchema").
		Msgf("%q declares %q as a ref with no target collection", id, field).
		Issue("collection", id).
		Issue("field", field).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "say which collection a ref field points to"})
}

// errRefUnknown is refused at declaration time rather than at render time: the
// view that would show the relation has nothing to resolve it against, and
// that is cheaper to catch now than to discover later.
func errRefUnknown(id, field, ref string) error {
	return apperr.New("COLLECTION_REF_UNKNOWN").
		Causer("collection.ValidateSchema").
		Msgf("%q's field %q points at %q, which does not exist", id, field, ref).
		Issue("collection", id).
		Issue("field", field).
		Issue("ref", ref).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "create the collection " + ref + " first, or correct the field's ref"})
}

func errNoFields(id string) error {
	return apperr.New("COLLECTION_NO_FIELDS").
		Causer("collection.ValidateSchema").
		Msgf("%q declares no fields", id).
		Issue("collection", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "a collection without fields describes no record; declare at least one"})
}

// errHookUnknown is a refusal, not a silent skip: a normalisation that quietly
// did not happen is a record that looks right and is not.
func errHookUnknown(id, action string) error {
	return apperr.New("COLLECTION_HOOK_UNKNOWN").
		Causer("collection.ApplyHooks").
		Msgf("%q declares a hook with action %q, which this build does not implement", id, action).
		Issue("collection", id).
		Issue("action", action).
		Issue("allowed", hookActions).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{
			Label: "use one of the four declarative actions in the issue \"allowed\": " + strings.Join(hookActions, ", ") +
				" — source code is not one of them; the path to logic beyond that is a Routine with an activity trigger",
		})
}

func errNotFound(id string, known []string) error {
	return apperr.New("COLLECTION_NOT_FOUND").
		Causer("collection.Service.Get").
		Msgf("no collection named %q", id).
		Issue("collection", id).
		Issue("known", known).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{
			Label:   "list the collections that exist before writing to this one",
			Command: build.Name + " collections list",
			Tool:    "collections_list",
		})
}

// errRecordNotFound names both the collection and the record: an agent asking
// for a row it typed wrong needs to know which of the two ids was the miss.
func errRecordNotFound(collectionID, id string) error {
	return apperr.New("COLLECTION_RECORD_NOT_FOUND").
		Causer("collection.RecordService").
		Msgf("collection %q has no record %q", collectionID, id).
		Issue("collection", collectionID).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "list the collection's records before reading, updating or deleting one by id"})
}

func errNameInvalid(id string) error {
	return apperr.New("COLLECTION_NAME_INVALID").
		Causer("collection.DescriptorFor").
		Msgf("%q is not usable as a collection id", id).
		Issue("id", id).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "an id is a path segment: lowercase letters, digits, hyphen and underscore only"})
}
