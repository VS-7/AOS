package collections

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// The struct tag that tells the engine where a field comes from.
//
//	collection:"path"        the value comes from a path placeholder named
//	                         after the field's json tag
//	collection:"path=taskId" the same, with an explicit placeholder name
//	collection:"content"     the value is the Markdown body
//
// Everything else is YAML front matter, mirroring the original's
// .omit({ id: true, content: true }) on every schema.
const tagName = "collection"

const (
	tagPath    = "path"
	tagContent = "content"
)

type plan struct {
	pathFields map[string]int // placeholder name → field index
	content    int            // field index, or -1
}

var planCache sync.Map // reflect.Type → *plan

func planFor(t reflect.Type) *plan {
	if cached, ok := planCache.Load(t); ok {
		return cached.(*plan)
	}
	p := &plan{pathFields: map[string]int{}, content: -1}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get(tagName)
		if tag == "" {
			continue
		}
		switch {
		case tag == tagContent:
			p.content = i
		case tag == tagPath:
			p.pathFields[jsonNameOf(f)] = i
		case strings.HasPrefix(tag, tagPath+"="):
			p.pathFields[strings.TrimPrefix(tag, tagPath+"=")] = i
		}
	}
	planCache.Store(t, p)
	return p
}

func jsonNameOf(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" || tag == "-" {
		return strings.ToLower(f.Name)
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return strings.ToLower(f.Name)
	}
	return name
}

// Decode parses a stored record: YAML front matter into the struct fields, the
// body into the field tagged collection:"content", and the path placeholders
// into the fields tagged collection:"path".
//
// A Markdown file without front matter is not an error: the whole file becomes
// the body. Hand-written files are a supported input, and refusing one because
// it lacks a "---" would make the format less editable than it promises to be.
func Decode[T any](data []byte, key Key, m Model[T]) (*T, error) {
	v := new(T)

	// A Record carries its schema in data, so there are no struct tags to
	// reflect over. The branch is here rather than behind an interface because
	// there is exactly one such type and naming it is clearer than a protocol
	// with one implementer.
	if rec, ok := any(v).(*Record); ok {
		if err := decodeRecord(rec, data, key, m.Name, m.Format); err != nil {
			return nil, err
		}
		return v, nil
	}

	rv := reflect.ValueOf(v).Elem()
	if rv.Kind() != reflect.Struct {
		return nil, errDecode(m.Name, key.String(), errNotAStruct)
	}
	p := planFor(rv.Type())

	switch m.Format {
	case FormatJSON:
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, v); err != nil {
				return nil, errDecode(m.Name, key.String(), err)
			}
		}
	default:
		front, body := splitFrontMatter(data)
		if len(bytes.TrimSpace(front)) > 0 {
			if err := yaml.Unmarshal(front, v); err != nil {
				return nil, errDecode(m.Name, key.String(), err)
			}
		}
		if p.content >= 0 {
			setString(rv.Field(p.content), string(body))
		}
	}

	for name, idx := range p.pathFields {
		setString(rv.Field(idx), key[name])
	}
	return v, nil
}

// Encode is the exact inverse of Decode. Fields tagged collection:"path" and
// collection:"content" are cleared before serialisation, so they never end up
// duplicated in the front matter: the path is their only home.
func Encode[T any](v *T, m Model[T]) ([]byte, error) {
	if rec, ok := any(v).(*Record); ok {
		return encodeRecord(rec, m.Name, m.Format)
	}

	rv := reflect.ValueOf(v).Elem()
	if rv.Kind() != reflect.Struct {
		return nil, errEncode(m.Name, errNotAStruct)
	}
	p := planFor(rv.Type())

	// A shallow copy is enough: only string fields are cleared, and clearing
	// them on the copy cannot reach the caller's value.
	cp := reflect.New(rv.Type()).Elem()
	cp.Set(rv)
	for _, idx := range p.pathFields {
		cp.Field(idx).SetZero()
	}
	body := ""
	if p.content >= 0 {
		body = rv.Field(p.content).String()
		cp.Field(p.content).SetZero()
	}
	record := cp.Addr().Interface()

	if m.Format == FormatJSON {
		out, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return nil, errEncode(m.Name, err)
		}
		return append(out, '\n'), nil
	}

	front, err := marshalYAML(record)
	if err != nil {
		return nil, errEncode(m.Name, err)
	}

	var b bytes.Buffer
	b.WriteString("---\n")
	b.Write(front)
	if !bytes.HasSuffix(front, []byte("\n")) {
		b.WriteByte('\n')
	}
	b.WriteString("---\n")
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.Bytes(), nil
}

// marshalYAML uses an explicit two-space indent, so that a record rewritten by
// a different Go version keeps producing the same bytes and golden files stay
// stable.
func marshalYAML(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// splitFrontMatter separates the YAML block from the Markdown body. Both CRLF
// and LF inputs are accepted: a file edited on Windows is still a valid record.
func splitFrontMatter(data []byte) (front, body []byte) {
	norm := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	norm = bytes.TrimPrefix(norm, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM, written by some editors
	if !bytes.HasPrefix(norm, []byte("---\n")) {
		return nil, norm
	}
	rest := norm[len("---\n"):]
	end := bytes.Index(rest, []byte("\n---"))
	if end < 0 {
		// An unterminated block is front matter for the whole file; YAML will
		// report what is wrong with it, which is a better message than "no
		// body found".
		return rest, nil
	}
	front = rest[:end+1]
	after := rest[end+len("\n---"):]
	after = bytes.TrimPrefix(after, []byte("\n"))
	return front, bytes.TrimPrefix(after, []byte("\n"))
}

func setString(f reflect.Value, s string) {
	if f.Kind() == reflect.String && f.CanSet() {
		f.SetString(s)
	}
}

// KeyOf reads the path placeholders back out of a record, which is how the
// engine knows where to write a value it was handed.
func KeyOf[T any](v *T) Key {
	if rec, ok := any(v).(*Record); ok {
		return rec.Key.Clone()
	}

	rv := reflect.ValueOf(v).Elem()
	if rv.Kind() != reflect.Struct {
		return Key{}
	}
	p := planFor(rv.Type())
	k := make(Key, len(p.pathFields))
	for name, idx := range p.pathFields {
		// An empty placeholder is not part of the key. A record type may carry
		// path fields for several patterns — a test probe does, and so will a
		// record that moves between a workspace and a skill — and only the
		// ones that are set identify this record.
		if v := rv.Field(idx).String(); v != "" {
			k[name] = v
		}
	}
	return k
}

// FieldOf reads a front-matter field by its JSON name, for List filters and
// ordering. It returns ok=false when the record has no such field.
func FieldOf[T any](v *T, name string) (any, bool) {
	if rec, ok := any(v).(*Record); ok {
		val, found := rec.Fields[name]
		return val, found
	}

	rv := reflect.ValueOf(v).Elem()
	if rv.Kind() != reflect.Struct {
		return nil, false
	}
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		if jsonNameOf(t.Field(i)) == name {
			return rv.Field(i).Interface(), true
		}
	}
	return nil, false
}

// WithoutBody returns a copy with the Markdown body cleared.
//
// The in-memory index holds records in this form. The original indexes
// everything; here a body can be tens of kilobytes — a task description, a
// skill document — while the workspace inventory only needs names, so caching
// bodies would spend memory per workspace for nothing.
func WithoutBody[T any](v *T) *T {
	if rec, ok := any(v).(*Record); ok {
		light := *rec
		light.Content = ""
		light.Key = rec.Key.Clone()
		// Fields is a map, and its values may themselves be maps or slices —
		// a "list" field, a JSON-format collection's nested object. A shallow
		// struct copy still shares all of that with the record the in-memory
		// index holds; CloneJSON is what keeps a caller's mutation of the
		// result from reaching into the index at any depth.
		if rec.Fields != nil {
			light.Fields, _ = CloneJSON(rec.Fields).(map[string]any)
		}
		return any(&light).(*T)
	}

	clone := *v
	rv := reflect.ValueOf(&clone).Elem()
	if rv.Kind() != reflect.Struct {
		return &clone
	}
	if p := planFor(rv.Type()); p.content >= 0 {
		rv.Field(p.content).SetZero()
	}
	return &clone
}

// decodeRecord is Decode for a dynamic collection. The key comes from the path
// and overwrites whatever the file claimed, for the same reason the native
// path does it: the location is the identity.
func decodeRecord(rec *Record, data []byte, key Key, name string, format Format) error {
	rec.Fields = map[string]any{}
	switch format {
	case FormatJSON:
		if len(bytes.TrimSpace(data)) > 0 {
			if err := json.Unmarshal(data, &rec.Fields); err != nil {
				return errDecode(name, key.String(), err)
			}
		}
	default:
		front, body := splitFrontMatter(data)
		if len(bytes.TrimSpace(front)) > 0 {
			if err := yaml.Unmarshal(front, &rec.Fields); err != nil {
				return errDecode(name, key.String(), err)
			}
		}
		rec.Content = string(body)
	}
	rec.Key = key.Clone()
	return nil
}

// encodeRecord is the exact inverse. The key is not serialised: the path is
// its only home, which is the same invariant Encode holds for a native's
// collection:"path" fields.
//
// The Markdown branch mirrors Encode byte for byte — same delimiters, same
// blank-line rule before the body, same forced trailing newline on the body —
// because splitFrontMatter has to read back whatever this writes. A Record
// that round-tripped only through its own format would be a second file
// format wearing the first one's extension.
//
// The forced newline is not cosmetic here: ADR-0004 means these files are
// meant to be versioned in git and edited by hand, and dynamic-collection
// records are the ones an agent will create in the greatest number. A file
// missing its final newline shows up as "\ No newline at end of file" in
// every diff — a permanent, avoidable blemish on exactly the files users will
// see most.
func encodeRecord(rec *Record, name string, format Format) ([]byte, error) {
	fields := rec.Fields
	if fields == nil {
		fields = map[string]any{}
	}
	if format == FormatJSON {
		data, err := json.MarshalIndent(fields, "", "  ")
		if err != nil {
			return nil, errEncode(name, err)
		}
		return append(data, '\n'), nil
	}

	front, err := marshalYAML(fields)
	if err != nil {
		return nil, errEncode(name, err)
	}

	var b bytes.Buffer
	b.WriteString("---\n")
	b.Write(front)
	if !bytes.HasSuffix(front, []byte("\n")) {
		b.WriteByte('\n')
	}
	b.WriteString("---\n")
	if rec.Content != "" {
		b.WriteString("\n")
		b.WriteString(rec.Content)
		if !strings.HasSuffix(rec.Content, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.Bytes(), nil
}
