package format

import (
	"strings"
)

// EncodeMarkdown renders a normalised value as Markdown, following the shape of
// the original's formatter: a flat object becomes a key/value table, an array of
// objects becomes a columnar table, and anything nested is split into sections
// headed by its dotted path.
func EncodeMarkdown(v any) string {
	return strings.TrimRight(markdown(v, nil), "\n")
}

func markdown(v any, path []string) string {
	if IsScalar(v) {
		if len(path) == 0 {
			return scalarTOON(v)
		}
		return heading(path) + scalarTOON(v)
	}

	if arr, ok := v.([]any); ok {
		if isArrayOfObjects(arr) {
			table := columnarTable(arr)
			if len(path) == 0 {
				return table
			}
			return heading(path) + table
		}
		items := make([]string, 0, len(arr))
		for _, item := range arr {
			items = append(items, "- "+scalarTOON(item))
		}
		body := strings.Join(items, "\n")
		if len(path) == 0 {
			return body
		}
		return heading(path) + body
	}

	obj := v.(*Object)
	if len(path) == 0 && isFlat(obj) {
		return kvTable(obj)
	}

	sections := make([]string, 0, obj.Len())
	for _, k := range obj.Keys() {
		val, _ := obj.Get(k)
		child := append(append([]string{}, path...), k)
		switch t := val.(type) {
		case *Object:
			if isFlat(t) {
				sections = append(sections, heading(child)+kvTable(t))
				continue
			}
			sections = append(sections, markdown(t, child))
		case []any:
			sections = append(sections, markdown(t, child))
		default:
			sections = append(sections, heading(child)+scalarTOON(t))
		}
	}
	return strings.Join(sections, "\n\n")
}

func heading(path []string) string { return "## " + strings.Join(path, ".") + "\n\n" }

func isFlat(o *Object) bool {
	for _, k := range o.Keys() {
		v, _ := o.Get(k)
		if !IsScalar(v) {
			return false
		}
	}
	return true
}

func isArrayOfObjects(arr []any) bool {
	if len(arr) == 0 {
		return false
	}
	for _, item := range arr {
		if _, ok := item.(*Object); !ok {
			return false
		}
	}
	return true
}

func kvTable(o *Object) string {
	rows := make([][]string, 0, o.Len())
	for _, k := range o.Keys() {
		v, _ := o.Get(k)
		rows = append(rows, []string{k, plain(v)})
	}
	return table([]string{"Key", "Value"}, rows)
}

func columnarTable(arr []any) string {
	var headers []string
	seen := map[string]bool{}
	for _, item := range arr {
		for _, k := range item.(*Object).Keys() {
			if !seen[k] {
				seen[k] = true
				headers = append(headers, k)
			}
		}
	}
	rows := make([][]string, 0, len(arr))
	for _, item := range arr {
		obj := item.(*Object)
		row := make([]string, 0, len(headers))
		for _, h := range headers {
			v, _ := obj.Get(h)
			row = append(row, plain(v))
		}
		rows = append(rows, row)
	}
	return table(headers, rows)
}

// table renders an aligned Markdown table. Alignment costs nothing and makes
// the raw text readable, which matters because this format is chosen by a human.
func table(headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i := range headers {
			if i < len(r) && len(r[i]) > widths[i] {
				widths[i] = len(r[i])
			}
		}
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		b.WriteString("|")
		for i := range headers {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			b.WriteString(" " + cell + strings.Repeat(" ", widths[i]-len(cell)) + " |")
		}
		b.WriteString("\n")
	}
	writeRow(headers)
	b.WriteString("|")
	for _, w := range widths {
		b.WriteString(strings.Repeat("-", w+2) + "|")
	}
	b.WriteString("\n")
	for _, r := range rows {
		writeRow(r)
	}
	return strings.TrimRight(b.String(), "\n")
}

// plain renders a value for a table cell, without the TOON quoting rules: a
// Markdown table is read by a human, and quotes around every word would be noise.
func plain(v any) string {
	if s, ok := v.(string); ok {
		return strings.ReplaceAll(s, "|", "\\|")
	}
	if !IsScalar(v) {
		return EncodeTOON(v)
	}
	return scalarTOON(v)
}
