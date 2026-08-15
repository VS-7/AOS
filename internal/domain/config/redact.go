package config

import (
	"reflect"
	"strings"
)

// RedactedMark is what replaces a secret whose value is too short to fingerprint.
const RedactedMark = "***"

// Redact returns a deep copy where every field tagged secret:"true" is replaced
// by a fingerprint: "***…a1b2" — enough to identify which key is configured,
// useless to whoever reads it.
//
// The copy is deep on purpose. A shallow copy of Config shares the backing
// array of agents.providers and the models map with the caller, so redacting
// in place would blank the live configuration the daemon is holding — the
// value would be redacted everywhere, including where it is needed to reach
// the provider.
//
// The tag is the mechanism, not a convention: adding a secret field to the
// struct redacts it automatically, and TestEverySecretFieldIsRedacted walks the
// type to prove no field was missed.
func Redact(c Config) Config {
	src := reflect.ValueOf(c)
	dst := reflect.New(src.Type()).Elem()
	redactInto(dst, src, false)
	return dst.Interface().(Config)
}

// redactInto deep-copies src into dst, replacing the value when the field it
// came from was tagged as a secret.
func redactInto(dst, src reflect.Value, isSecret bool) {
	switch src.Kind() {
	case reflect.String:
		if isSecret {
			dst.SetString(Fingerprint(src.String()))
			return
		}
		dst.SetString(src.String())

	case reflect.Struct:
		t := src.Type()
		for i := 0; i < src.NumField(); i++ {
			if !dst.Field(i).CanSet() {
				continue
			}
			redactInto(dst.Field(i), src.Field(i), t.Field(i).Tag.Get("secret") == "true")
		}

	case reflect.Slice:
		if src.IsNil() {
			return
		}
		out := reflect.MakeSlice(src.Type(), src.Len(), src.Cap())
		for i := 0; i < src.Len(); i++ {
			redactInto(out.Index(i), src.Index(i), isSecret)
		}
		dst.Set(out)

	case reflect.Array:
		for i := 0; i < src.Len(); i++ {
			redactInto(dst.Index(i), src.Index(i), isSecret)
		}

	case reflect.Map:
		if src.IsNil() {
			return
		}
		out := reflect.MakeMapWithSize(src.Type(), src.Len())
		for _, k := range src.MapKeys() {
			elem := reflect.New(src.Type().Elem()).Elem()
			redactInto(elem, src.MapIndex(k), isSecret)
			out.SetMapIndex(k, elem)
		}
		dst.Set(out)

	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		out := reflect.New(src.Type().Elem())
		redactInto(out.Elem(), src.Elem(), isSecret)
		dst.Set(out)

	default:
		dst.Set(src)
	}
}

// Fingerprint reduces a secret to something identifiable but unusable: the last
// four characters behind the mark. An empty value stays empty, because "not
// configured" and "configured but hidden" must remain distinguishable.
func Fingerprint(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return RedactedMark
	}
	return RedactedMark + "…" + s[len(s)-4:]
}

// SecretPaths returns the dotted path of every secret field of Config, derived
// from the tags. The error catalog test uses it to prove no error puts one of
// these into an Issue.
func SecretPaths() []string {
	var out []string
	collectSecretPaths(reflect.TypeOf(Config{}), "", &out)
	return out
}

func collectSecretPaths(t reflect.Type, prefix string, out *[]string) {
	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := jsonName(f.Tag.Get("json"), f.Name)
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			if f.Tag.Get("secret") == "true" {
				*out = append(*out, path)
				continue
			}
			collectSecretPaths(f.Type, path, out)
		}
	case reflect.Slice, reflect.Array:
		collectSecretPaths(t.Elem(), prefix+"[]", out)
	case reflect.Map:
		collectSecretPaths(t.Elem(), prefix+"{}", out)
	case reflect.Pointer:
		collectSecretPaths(t.Elem(), prefix, out)
	}
}

func jsonName(tag, fallback string) string {
	if tag == "" {
		return fallback
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" || name == "-" {
		return fallback
	}
	return name
}
