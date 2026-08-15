package command

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = func() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Report violations by the JSON name, because that is the name every
	// surface uses: the CLI flag, the schema property and the HTTP body field.
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		if name := jsonNameOf(f); name != "" {
			return name
		}
		return f.Name
	})
	return v
}()

func validateStruct(in any) error { return validate.Struct(in) }

func validateStructExcept(in any, field string) error {
	return validate.StructExcept(in, field)
}

// checkTags enforces the contract every input struct must satisfy: a json name
// and a jsonschema description on every exported field.
//
// Reflection over struct tags fails silently otherwise — a field without a json
// tag becomes a flag named after the Go field, which changes the published
// surface without any error at all.
func checkTags(t reflect.Type) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var problems []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // unexported
		}
		if f.Anonymous {
			if err := checkTags(f.Type); err != nil {
				problems = append(problems, err.Error())
			}
			continue
		}
		name := jsonNameOf(f)
		if name == "" {
			problems = append(problems, fmt.Sprintf("field %s has no json tag", f.Name))
			continue
		}
		if f.Tag.Get("jsonschema") == "" {
			problems = append(problems, fmt.Sprintf(
				"field %s has no jsonschema description — it is what the model reads to fill the payload", f.Name))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

// translateValidation turns a validator failure into an app error carrying the
// path to introspect the contract.
//
// This follows the rule the master prompt gives the agent: "If a call fails
// validation, do NOT retry blindly: read the error, inspect the contract with
// schema: true, then fix." The error carries that path already built.
func translateValidation(key string, err error) error {
	var invalid *validator.InvalidValidationError
	if ok := asInvalid(err, &invalid); ok {
		return errInvalidInput(key, err)
	}

	var fields validator.ValidationErrors
	if !asValidationErrors(err, &fields) {
		return errInvalidInput(key, err)
	}

	e := errValidation(key)
	for _, f := range fields {
		if f.Field() == ReasoningField {
			e = e.Issue(f.Field(), ReasoningRejection)
			continue
		}
		e = e.Issue(f.Field(), describeRule(f))
	}
	return e
}

func describeRule(f validator.FieldError) string {
	switch f.Tag() {
	case "required":
		return "is required"
	case "min":
		return "must be at least " + f.Param()
	case "max":
		return "must be at most " + f.Param()
	case "gte":
		return "must be greater than or equal to " + f.Param()
	case "lte":
		return "must be less than or equal to " + f.Param()
	case "oneof":
		return "must be one of: " + strings.ReplaceAll(f.Param(), " ", ", ")
	case "url":
		return "must be a URL"
	case "email":
		return "must be an email address"
	default:
		return "fails the rule " + f.Tag()
	}
}

func asInvalid(err error, target **validator.InvalidValidationError) bool {
	v, ok := err.(*validator.InvalidValidationError)
	if ok {
		*target = v
	}
	return ok
}

func asValidationErrors(err error, target *validator.ValidationErrors) bool {
	v, ok := err.(validator.ValidationErrors)
	if ok {
		*target = v
	}
	return ok
}
