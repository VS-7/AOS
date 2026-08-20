package view

import "github.com/OWNER/aos/internal/core/apperr"

// Every refusal here names both what is wrong and where in the tree it is:
// the "at" issue carries a path like "tree.children[2].children[0]", because
// a view of thirty nodes whose error does not locate the bad one is an error
// the agent cannot act on. This is the whole point of the domain: the
// original validates while rendering, so a mistake surfaces as a blank
// screen; here it surfaces as a refusal, in the same turn the agent made it.

func errComponentUnknown(at, component string) error {
	return apperr.New("VIEW_COMPONENT_UNKNOWN").
		Causer("view.Validate").
		Msgf("%q at %s is not a component the catalog declares", component, at).
		Issue("at", at).
		Issue("component", component).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "call Components to see what is available", Tool: "view_components"})
}

func errPropRequired(at, component, prop string) error {
	return apperr.New("VIEW_PROP_REQUIRED").
		Causer("view.Validate").
		Msgf("%s (%s) does not give a value for its required prop %q, and nothing binds it", at, component, prop).
		Issue("at", at).
		Issue("component", component).
		Issue("prop", prop).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "set " + prop + " in props, or bind it to a field of the source collection"})
}

func errPropWrongType(at, component, prop, want string) error {
	return apperr.New("VIEW_PROP_WRONG_TYPE").
		Causer("view.Validate").
		Msgf("%s (%s) gives %q a value that is not %s", at, component, prop, want).
		Issue("at", at).
		Issue("component", component).
		Issue("prop", prop).
		Issue("expected", want).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "send " + prop + " as " + want})
}

// errPropNotInEnum is refused for the same reason a wrong type is: the design
// system rejects a value outside its enum, and letting that reach the
// frontend renders wrong instead of not rendering at all — the promise this
// domain exists to keep. "allowed" carries the accepted values because the
// agent's next move is to pick one of them.
func errPropNotInEnum(at, component, prop string, got any, allowed []any) error {
	return apperr.New("VIEW_PROP_NOT_IN_ENUM").
		Causer("view.Validate").
		Msgf("%s (%s) gives %q the value %v, which is not one of the values the component accepts", at, component, prop, got).
		Issue("at", at).
		Issue("component", component).
		Issue("prop", prop).
		Issue("given", got).
		Issue("allowed", allowed).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use one of the values in the issue \"allowed\""})
}

// errBindUnknownField is refused at write time rather than left to render:
// the original renders the bind against nothing and shows nothing, silently.
func errBindUnknownField(at, field, collectionID string) error {
	return apperr.New("VIEW_BIND_UNKNOWN_FIELD").
		Causer("view.Validate").
		Msgf("%s binds a prop to %q, which %q does not declare", at, field, collectionID).
		Issue("at", at).
		Issue("field", field).
		Issue("collection", collectionID).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "bind to one of the source collection's declared fields"})
}

func errChildrenNotAccepted(at, component string) error {
	return apperr.New("VIEW_CHILDREN_NOT_ACCEPTED").
		Causer("view.Validate").
		Msgf("%s (%s) declares no slots, so it cannot have children", at, component).
		Issue("at", at).
		Issue("component", component).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "move the children under a container component such as Stack or Card"})
}

// errActionCommandUnknown is what keeps a view from being a second path to
// mutation: an action is a Descriptor from the registry, checked with the
// same rigour as any CLI invocation or MCP tool call, not a free-form call to
// anything named in a button.
func errActionCommandUnknown(at, command string) error {
	return apperr.New("VIEW_ACTION_COMMAND_UNKNOWN").
		Causer("view.Validate").
		Msgf("%s names the command %q, which is not in the registry", at, command).
		Issue("at", at).
		Issue("command", command).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "name a command the registry knows, or register the command first"})
}

func errNotFound(id string) error {
	return apperr.New("VIEW_NOT_FOUND").
		Causer("view.Service").
		Msgf("no view named %q", id).
		Issue("id", id).
		Status(apperr.StatusNotFound).
		CTA(apperr.CallToAction{Label: "list the views that exist before reading, rendering or deleting one by id", Tool: "view_list"})
}

func errIDRequired() error {
	return apperr.New("VIEW_ID_REQUIRED").
		Causer("view.Service.Create").
		Msgf("a view needs an id; it is also its file name").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "give the view an id: lowercase letters, digits, hyphen and underscore"})
}

// errActionNotDeclared is not the same refusal as an unregistered command:
// this action names a command that exists, but the view itself never declared
// it. Accepting it anyway would make a view a way to call anything in the
// registry from a button nobody wrote into the tree.
func errActionNotDeclared(id, label string) error {
	return apperr.New("VIEW_ACTION_NOT_DECLARED").
		Causer("view.Service.ExecuteAction").
		Msgf("view %q declares no action labelled %q", id, label).
		Issue("id", id).
		Issue("label", label).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "use one of the labels declared on the view's own actions"})
}

// errActionInputInvalid guards json.Marshal of the merged action input. A
// map[string]any built by merging two maps of ordinary JSON values cannot
// realistically fail to marshal; the check exists because errcheck requires
// it, and because "cannot realistically fail" is not the same as "cannot
// fail".
func errActionInputInvalid(id, label string, cause error) error {
	return apperr.New("VIEW_ACTION_INPUT_INVALID").
		Causer("view.Service.ExecuteAction").
		Msgf("the input for action %q on view %q does not encode as JSON: %v", label, id, cause).
		Issue("id", id).
		Issue("label", label).
		Status(apperr.StatusBadRequest).
		Wrap(cause).
		CTA(apperr.CallToAction{Label: "pass only JSON-representable values in the action's input"})
}
