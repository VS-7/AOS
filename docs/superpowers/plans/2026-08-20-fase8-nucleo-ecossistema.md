# Fase 8, núcleo do Ecossistema — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fazer o AOS deixar de ter domínios fixos — o agente define dados estruturados em runtime, compõe uma interface sobre eles, alcança ferramentas externas, e tudo isso vira um pacote instalável.

**Architecture:** Uma fatia vertical fina pelos quatro domínios (`collection`, `view`, `toolset`, `skill`). O motor de coleções ganha um `Record` — mapa mais `Key`, com seu ramo no codec — e um registry por instância com dinâmicos registráveis, para reusar escrita atômica, lock, CAS e casamento de padrões em vez de escrever um segundo caminho de persistência. O frontend dos quatro já está portado e dormente: acender é apagar entrada de `DORMANT_DOMAINS` e preencher `COMMAND_MAP`.

**Tech Stack:** Go 1.x com `internal/core/collections` (motor próprio), `github.com/modelcontextprotocol/go-sdk` (cliente MCP), Node/`tsx` + `zod-to-json-schema` para a única geração que anda de TS para Go.

**Spec:** `docs/superpowers/specs/2026-08-20-fase8-nucleo-ecossistema-design.md`

## Global Constraints

- **Módulo:** `github.com/OWNER/aos`. Todo import interno começa assim.
- **Diretório de estado:** `collections.Root`, que é `build.StateDir` = `.aos`. Nunca escreva `.aos` literal num padrão — use `Root+"/..."`, como `registry.go` já faz.
- **`internal/domain` não importa `os`, `os/exec` nem `net/http`.** É regra executável: `TestDependencyRule` em `internal/architecture` falha. Tudo que toca IO entra por port e vive em `internal/adapters/`.
- **Teste de domínio não toca IO.** `TestDomainTestsDoNotTouchIO` afirma isso. Domínio testa contra fake; disco real só em `internal/app` e em `internal/adapters`.
- **Nada de `time.Now()`, `math/rand` ou `context.TODO()` em código de produção.** `forbidigo` recusa. Injete `clockx.Clock`, `ids.IDs`. O único call site permitido de `time.Now` é `internal/core/clockx`.
- **Todo `Input` de comando embute `command.Reasoning`**, cujo campo `_reasoning` é obrigatório e não-vazio.
- **Todo erro é `apperr.New("CÓDIGO")`** com `.Causer()`, `.Msgf()`, `.Status()` e, quando houver ação possível, `.CTA()`. O prefixo `AOS_` é acrescentado na leitura — declare sem ele, asserte com ele.
- **`errcheck`, `noctx`, `misspell`, `gocritic`, `revive`, `errorlint`, `bodyclose`, `unconvert`, `wastedassign` estão ligados.** `defer f.Close()` com retorno ignorado precisa ser `defer func() { _ = f.Close() }()`.
- **Piso de cobertura por pacote.** `task cover` falha se um pacote novo ficar abaixo do seu piso. Domínio novo nasce com teste, não com dívida.
- **Gates de cada task:** `task vet`, `task lint`, `task test`, e no fim de tasks que mexem em gerado, `task check` inteiro.

---

### Task 1: `collections.Record` — o T das coleções definidas em runtime

**Files:**
- Modify: `internal/core/collections/model.go` (acrescenta o tipo `Record`)
- Modify: `internal/core/collections/codec.go` (ramos de `Decode`, `Encode`, `KeyOf`, `FieldOf`, `WithoutBody`)
- Test: `internal/core/collections/record_test.go`

**Interfaces:**
- Consumes: nada. É a primeira task.
- Produces: `collections.Record` com campos `Key Key`, `Fields map[string]any`, `Content string`. `Decode[Record]`, `Encode[Record]`, `KeyOf[Record]`, `FieldOf[Record]`, `WithoutBody[Record]` passam a funcionar. Tasks 4, 5 e 7 dependem disso.

**Contexto que o implementador precisa:** `Decode[T]`/`Encode[T]` hoje passam por `planFor(reflect.Type)`, que lê as tags `collection:"path"` e `collection:"content"` de um struct. Um `Record` não tem campos tagueados — os campos dele são dados dentro de um mapa. O ramo novo é um type assertion, checado **antes** da reflexão.

- [ ] **Step 1: Write the failing test**

Crie `internal/core/collections/record_test.go`:

```go
package collections_test

import (
	"testing"

	"github.com/OWNER/aos/internal/core/collections"
)

// A dynamic collection has no Go struct: its fields were declared by an agent
// at runtime and live in a map. The engine still has to round-trip it, because
// every guarantee around a record — atomic write, the per-file lock, the CAS
// check — is the same one the native collections get.
func TestARecordRoundTripsThroughMarkdown(t *testing.T) {
	model := collections.Model[collections.Record]{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
	in := &collections.Record{
		Key:     collections.Key{"id": "ada"},
		Fields:  map[string]any{"name": "Ada Lovelace", "stage": "won", "score": 42},
		Content: "Conheceu o Babbage numa festa.",
	}

	raw, err := collections.Encode(in, model)
	if err != nil {
		t.Fatal(err)
	}
	out, err := collections.Decode(raw, collections.Key{"id": "ada"}, model)
	if err != nil {
		t.Fatal(err)
	}

	if out.Key["id"] != "ada" {
		t.Fatalf("key = %v, want id=ada", out.Key)
	}
	if out.Fields["name"] != "Ada Lovelace" || out.Fields["stage"] != "won" {
		t.Fatalf("fields = %v", out.Fields)
	}
	if out.Content != "Conheceu o Babbage numa festa." {
		t.Fatalf("content = %q", out.Content)
	}
}

// The key lives in the path and nowhere else — the same rule Encode applies to
// a native record's `collection:"path"` fields. A key duplicated into the front
// matter is a second source of truth that drifts the first time a file moves.
func TestARecordsKeyIsNotWrittenIntoTheFrontMatter(t *testing.T) {
	model := collections.Model[collections.Record]{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
	raw, err := collections.Encode(&collections.Record{
		Key:    collections.Key{"id": "ada"},
		Fields: map[string]any{"name": "Ada"},
	}, model)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); contains(got, "id: ada") {
		t.Fatalf("the key was duplicated into the front matter:\n%s", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

func TestARecordRoundTripsThroughJSON(t *testing.T) {
	model := collections.Model[collections.Record]{
		Name:     "deals",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/deals/records/{id}.json")},
		Format:   collections.FormatJSON,
	}
	in := &collections.Record{
		Key:    collections.Key{"id": "d1"},
		Fields: map[string]any{"title": "Contrato anual", "amount": 1200.5, "open": true},
	}

	raw, err := collections.Encode(in, model)
	if err != nil {
		t.Fatal(err)
	}
	out, err := collections.Decode(raw, collections.Key{"id": "d1"}, model)
	if err != nil {
		t.Fatal(err)
	}
	if out.Fields["title"] != "Contrato anual" || out.Fields["open"] != true {
		t.Fatalf("fields = %v", out.Fields)
	}
}

// KeyOf and FieldOf are what List, Query.Filters and the write path call. A
// Record that answered neither would be a record the engine cannot index.
func TestKeyOfAndFieldOfReadARecordsMap(t *testing.T) {
	r := &collections.Record{
		Key:    collections.Key{"id": "ada", "collection": "contacts"},
		Fields: map[string]any{"stage": "won"},
	}

	k := collections.KeyOf(r)
	if k["id"] != "ada" || k["collection"] != "contacts" {
		t.Fatalf("KeyOf = %v", k)
	}
	if v, ok := collections.FieldOf(r, "stage"); !ok || v != "won" {
		t.Fatalf("FieldOf(stage) = %v, %v", v, ok)
	}
	if _, ok := collections.FieldOf(r, "nope"); ok {
		t.Fatal("FieldOf reported a field that does not exist")
	}
}

// WithoutBody is what List returns when IncludeContent is off: the answer to
// "what is here", not "what does it say".
func TestWithoutBodyDropsARecordsContent(t *testing.T) {
	r := &collections.Record{
		Key:     collections.Key{"id": "ada"},
		Fields:  map[string]any{"name": "Ada"},
		Content: "um corpo longo",
	}
	light := collections.WithoutBody(r)
	if light.Content != "" {
		t.Fatalf("content = %q, want empty", light.Content)
	}
	if r.Content == "" {
		t.Fatal("WithoutBody mutated the original")
	}
	if light.Fields["name"] != "Ada" {
		t.Fatalf("WithoutBody dropped the fields too: %v", light.Fields)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/collections/ -run TestARecord -v`
Expected: FAIL — `undefined: collections.Record`

- [ ] **Step 3: Add the Record type**

Em `internal/core/collections/model.go`, logo depois da declaração de `Key`:

```go
// Record is the T of a collection whose fields were declared at runtime rather
// than as a Go struct.
//
// Everything the engine does around a record — matching a path to a pattern,
// choosing a writable pattern for a key, the atomic write, the per-file lock,
// the CAS check on Version, the Changed event — looks at the pattern and the
// key, never inside T. That is what lets a collection an agent invented at
// 14:00 be a Model[Record] instead of a second engine.
type Record struct {
	// Key is the placeholder values that identify this record, exactly as for
	// a native: it lives in the path and is never written into the body.
	Key Key

	// Fields is the declared data. internal/domain/collection validates it
	// against the collection's schema before it ever reaches here — the engine
	// stores what it is given.
	Fields map[string]any

	// Content is the Markdown body, for a collection with FormatMarkdown. A
	// FormatJSON collection leaves it empty.
	Content string
}
```

- [ ] **Step 4: Add the codec branches**

Em `internal/core/collections/codec.go`, dentro de `Decode`, **antes** de `rv := reflect.ValueOf(v).Elem()`:

```go
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
```

E em `Encode`, antes da mesma linha:

```go
	if rec, ok := any(v).(*Record); ok {
		return encodeRecord(rec, m.Name, m.Format)
	}
```

Acrescente ao fim de `codec.go`:

```go
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

// encodeRecord is the exact inverse. The key is not serialised: the path is its
// only home, which is the same invariant Encode holds for a native's
// collection:"path" fields.
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
	out := make([]byte, 0, len(front)+len(rec.Content)+16)
	out = append(out, "---\n"...)
	out = append(out, front...)
	out = append(out, "---\n\n"...)
	out = append(out, rec.Content...)
	return out, nil
}
```

**Cuidado:** confira como `Encode` monta o front matter para um nativo e **espelhe exatamente** o formato de delimitadores e quebras de linha que `splitFrontMatter` sabe ler. Se o nativo usa outro espaçamento, use o dele — um round-trip que só funciona para `Record` é um formato novo, não o mesmo formato.

- [ ] **Step 5: Add the KeyOf, FieldOf and WithoutBody branches**

No topo de cada uma das três funções em `codec.go`:

```go
// KeyOf
	if rec, ok := any(v).(*Record); ok {
		return rec.Key.Clone()
	}

// FieldOf
	if rec, ok := any(v).(*Record); ok {
		val, found := rec.Fields[name]
		return val, found
	}

// WithoutBody
	if rec, ok := any(v).(*Record); ok {
		light := *rec
		light.Content = ""
		light.Key = rec.Key.Clone()
		return any(&light).(*T)
	}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/core/collections/ -run "TestARecord|TestKeyOf|TestWithoutBody" -v`
Expected: PASS, todos.

- [ ] **Step 7: Run the whole engine suite — nothing native may have changed**

Run: `go test -race ./internal/core/collections/... ./internal/adapters/fscollections/...`
Expected: PASS. Se um teste nativo quebrou, o ramo de `Record` foi posto no lugar errado — ele tem que vir **antes** da reflexão e não pode tocar o caminho genérico.

- [ ] **Step 8: Commit**

```bash
git add internal/core/collections/model.go internal/core/collections/codec.go internal/core/collections/record_test.go
git commit -m "feat(collections): Record, the T of a collection declared at runtime"
```

---

### Task 2: O registry por instância

**Files:**
- Modify: `internal/core/collections/registry.go`
- Test: `internal/core/collections/registry_test.go` (acrescenta ao existente)

**Interfaces:**
- Consumes: `collections.Descriptor` (já existe), `collections.Record` (Task 1).
- Produces: `*collections.Registry` com `NewRegistry() *Registry`, `(*Registry).Register(Descriptor) error`, `(*Registry).Unregister(name string) error`, `(*Registry).Lookup(name string) (Descriptor, bool)`, `(*Registry).Names() []string`, `(*Registry).IsNative(name string) bool`. As funções de pacote `Lookup`, `Natives` e `ModelOf` continuam com a assinatura de hoje. Tasks 4, 5, 6 e 10 dependem disso.

**Contexto:** hoje `byName` é um `var` de pacote computado uma vez a partir de `natives`, e `Lookup`/`ModelOf` leem esse estado. Uma coleção que o agente cria às 14h tem que ser consultável às 14h01 na mesma sessão. Estado de pacote mutável seria racy; a saída é uma instância com `RWMutex`, e as funções de pacote delegando a uma default para que os 14 domínios existentes não mudem uma linha.

- [ ] **Step 1: Write the failing test**

Acrescente a `internal/core/collections/registry_test.go`:

```go
// A collection an agent invents has to be usable in the same session — that is
// the whole point of the original's autoWatch. The registry is therefore an
// instance with a lock, not package state computed once at init.
func TestADynamicCollectionIsVisibleAsSoonAsItIsRegistered(t *testing.T) {
	reg := collections.NewRegistry()

	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("contacts existed before anybody registered it")
	}
	desc := collections.Descriptor{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
	if err := reg.Register(desc); err != nil {
		t.Fatal(err)
	}
	got, ok := reg.Lookup("contacts")
	if !ok {
		t.Fatal("the collection is not there right after being registered")
	}
	if got.Name != "contacts" {
		t.Fatalf("name = %q", got.Name)
	}
}

// A custom collection that shadowed a native one would break the engine's own
// registry: two things would claim the same name and the loser would be
// whichever was asked for second.
func TestARegisteredNameMayNotShadowANative(t *testing.T) {
	reg := collections.NewRegistry()

	for _, name := range []string{"agents", "skills", "memories", "tasks", "chats", "routines"} {
		err := reg.Register(collections.Descriptor{
			Name:     name,
			Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/x/records/{id}.md")},
		})
		if err == nil {
			t.Fatalf("registering %q as a custom collection was allowed", name)
		}
		if code := codeOfErr(t, err); code != "AOS_COLLECTION_NAME_RESERVED" {
			t.Fatalf("%s: code = %q, want AOS_COLLECTION_NAME_RESERVED", name, code)
		}
	}
}

func TestANativeCannotBeUnregistered(t *testing.T) {
	reg := collections.NewRegistry()

	if err := reg.Unregister("agents"); err == nil {
		t.Fatal("a native collection was unregistered")
	}
	if _, ok := reg.Lookup("agents"); !ok {
		t.Fatal("the native disappeared anyway")
	}
}

func TestUnregisteringADynamicRemovesIt(t *testing.T) {
	reg := collections.NewRegistry()
	desc := collections.Descriptor{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
	}
	if err := reg.Register(desc); err != nil {
		t.Fatal(err)
	}
	if err := reg.Unregister("contacts"); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the collection survived being unregistered")
	}
}

// Registering the same name twice is the uninstall-then-reinstall path of a
// skill-scoped collection. It replaces rather than failing, because failing
// would leave a workspace that cannot reinstall what it just removed.
func TestRegisteringTheSameNameTwiceReplaces(t *testing.T) {
	reg := collections.NewRegistry()
	first := collections.Descriptor{
		Name:     "contacts",
		Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
		Format:   collections.FormatMarkdown,
	}
	second := first
	second.Format = collections.FormatJSON
	second.Patterns = []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.json")}

	if err := reg.Register(first); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(second); err != nil {
		t.Fatal(err)
	}
	got, _ := reg.Lookup("contacts")
	if got.Format != collections.FormatJSON {
		t.Fatalf("format = %v, want the second registration to win", got.Format)
	}
}

// The registry is read by whatever is running and written by the watcher and by
// the create path. An unguarded map races on the first test that exercises one.
func TestTheRegistryIsSafeUnderConcurrentUse(t *testing.T) {
	reg := collections.NewRegistry()
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			_ = reg.Register(collections.Descriptor{
				Name:     "contacts",
				Patterns: []*collections.Pattern{collections.MustCompile(collections.Root + "/collections/contacts/records/{id}.md")},
			})
			_ = reg.Unregister("contacts")
		}
	}()
	for i := 0; i < 200; i++ {
		reg.Lookup("contacts")
		reg.Names()
	}
	<-done
}

func codeOfErr(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	return app.Code
}
```

Acrescente os imports `errors` e `github.com/OWNER/aos/internal/core/apperr` ao arquivo de teste, se ainda não estiverem lá.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/collections/ -run "TestADynamic|TestARegisteredName|TestANative|TestUnregistering|TestRegisteringTheSame|TestTheRegistryIs" -v`
Expected: FAIL — `undefined: collections.NewRegistry`

- [ ] **Step 3: Add the Registry type**

Em `internal/core/collections/registry.go`, depois de `Natives()`:

```go
// Registry holds what collections exist.
//
// The natives are fixed at construction. The dynamic ones come and go while
// the daemon runs, which is the entire point of a collection an agent can
// create: the original's watcher loads a schema the moment it appears, and a
// collection you have to restart to use is a collection the agent cannot use
// in the turn that created it.
//
// It is an instance rather than mutable package state because it is read by
// whatever is running and written by the watcher — an unguarded package map
// races on the first turn that creates a collection while a list is in flight.
type Registry struct {
	mu      sync.RWMutex
	natives map[string]Descriptor
	dynamic map[string]Descriptor
}

// NewRegistry returns a registry holding the native collections and nothing
// else.
func NewRegistry() *Registry {
	r := &Registry{
		natives: make(map[string]Descriptor, len(natives)),
		dynamic: map[string]Descriptor{},
	}
	for _, d := range natives {
		r.natives[d.Name] = d
	}
	return r
}

// Register adds or replaces a dynamic collection.
//
// Replacing rather than refusing a duplicate is deliberate: uninstalling and
// reinstalling a skill re-registers the collections it ships, and a registry
// that refused the second registration would leave a workspace unable to
// reinstall what it had just removed.
func (r *Registry) Register(d Descriptor) error {
	if d.Name == "" {
		return errCollectionNameEmpty()
	}
	if len(d.Patterns) == 0 {
		return errCollectionNoPatterns(d.Name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, native := r.natives[d.Name]; native {
		return errCollectionNameReserved(d.Name)
	}
	r.dynamic[d.Name] = d
	return nil
}

// Unregister removes a dynamic collection. A native name is refused: the
// engine's own collections are not the caller's to remove, and letting one go
// would break every domain built on it.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, native := r.natives[name]; native {
		return errCollectionNativeNotRemovable(name)
	}
	delete(r.dynamic, name)
	return nil
}

// Lookup returns a descriptor by name, native or dynamic.
func (r *Registry) Lookup(name string) (Descriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if d, ok := r.natives[name]; ok {
		return d, true
	}
	d, ok := r.dynamic[name]
	return d, ok
}

// IsNative reports whether a name belongs to the engine rather than to a
// collection somebody declared.
func (r *Registry) IsNative(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.natives[name]
	return ok
}

// Names returns every collection name, natives first, each group sorted, so a
// listing is stable between calls and between machines.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.natives)+len(r.dynamic))
	for name := range r.natives {
		out = append(out, name)
	}
	sort.Strings(out)
	dyn := make([]string, 0, len(r.dynamic))
	for name := range r.dynamic {
		dyn = append(dyn, name)
	}
	sort.Strings(dyn)
	return append(out, dyn...)
}
```

Acrescente `"sync"` aos imports do arquivo.

- [ ] **Step 4: Add the three errors**

Em `internal/core/collections/errors.go`:

```go
func errCollectionNameEmpty() error {
	return apperr.New("COLLECTION_NAME_EMPTY").
		Causer("collections.Registry.Register").
		Msgf("a collection must have a name").
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "give the collection a name; it is what every record path is built from"})
}

func errCollectionNoPatterns(name string) error {
	return apperr.New("COLLECTION_NO_PATTERNS").
		Causer("collections.Registry.Register").
		Msgf("%q was registered without a pattern, so no record could ever be written", name).
		Issue("collection", name).
		Status(apperr.StatusBadRequest).
		CTA(apperr.CallToAction{Label: "declare at least one writable pattern for the collection"})
}

// errCollectionNameReserved is why the reserved list lives here and not in the
// domain: the registry is what knows which names are taken, and a copy of the
// list kept next to the domain goes stale the day a native is added.
func errCollectionNameReserved(name string) error {
	return apperr.New("COLLECTION_NAME_RESERVED").
		Causer("collections.Registry.Register").
		Msgf("%q is a built-in collection and cannot be redefined", name).
		Issue("collection", name).
		Issue("reserved", nativeNames()).
		Status(apperr.StatusConflict).
		CTA(apperr.CallToAction{Label: "choose a name that is not one of the built-in collections"})
}

func errCollectionNativeNotRemovable(name string) error {
	return apperr.New("COLLECTION_NATIVE_NOT_REMOVABLE").
		Causer("collections.Registry.Unregister").
		Msgf("%q is a built-in collection and cannot be removed", name).
		Issue("collection", name).
		Status(apperr.StatusForbidden).
		CTA(apperr.CallToAction{Label: "only collections declared at runtime can be unregistered"})
}
```

`nativeNames()` já existe em `errors.go` — reuse-a, não escreva uma segunda lista.

- [ ] **Step 5: Point the package-level functions at a default registry**

Ainda em `registry.go`, substitua o `var byName` por:

```go
// defaultRegistry is what the package-level Lookup, Natives and ModelOf read.
// It exists so the fourteen domains written before this type do not change a
// line: they ask the package, the package asks the default instance.
var defaultRegistry = NewRegistry()

// Default returns the process-wide registry. A caller that needs to register a
// dynamic collection takes this, or is given its own instance — the wiring in
// internal/app passes one explicitly.
func Default() *Registry { return defaultRegistry }
```

E reescreva `Lookup` de pacote como `func Lookup(name string) (Descriptor, bool) { return defaultRegistry.Lookup(name) }`.

**Cuidado:** o `panic` de nome duplicado que hoje vive na construção de `byName` tem que continuar existindo — mova-o para dentro de `NewRegistry`, porque um nativo declarado duas vezes é erro de programação e falhar cedo é o comportamento certo.

- [ ] **Step 6: Run the tests**

Run: `go test -race ./internal/core/collections/ -v`
Expected: PASS, incluindo os testes antigos de registry sem alteração.

- [ ] **Step 7: Prove nothing else moved**

Run: `task vet && task test`
Expected: toda a árvore verde. Os 14 domínios chamam `collections.Lookup` e `collections.ModelOf` — se algum quebrou, a delegação está errada.

- [ ] **Step 8: Commit**

```bash
git add internal/core/collections/registry.go internal/core/collections/errors.go internal/core/collections/registry_test.go
git commit -m "feat(collections): a registry that exists at runtime, not only at init"
```

---

### Task 3: O catálogo de componentes, gerado do frontend

**Files:**
- Create: `frontend/scripts/gen-components.mjs`
- Create: `internal/domain/view/components.json` (gerado, commitado)
- Create: `internal/domain/view/catalog.go`
- Create: `internal/domain/view/catalog_test.go`
- Modify: `frontend/package.json` (devDependencies: `tsx`, `zod-to-json-schema`)
- Modify: `Taskfile.yml` (task `gen-components`, entrada em `gen`, guarda em `check`)

**Interfaces:**
- Consumes: nada de tasks anteriores.
- Produces: `view.ComponentSpec` com campos `Name string`, `Description string`, `Category string`, `Props map[string]any` (JSON Schema), `Slots []string`, `AcceptsChildren bool`; e `view.Catalog() []ComponentSpec`, `view.LookupComponent(name string) (ComponentSpec, bool)`. Task 7 depende disso.

**Contexto:** esta é a única geração do projeto que anda de TypeScript para Go. As outras três são programas Go (`gencatalog` lê Go, `gentokens` lê CSS como texto, `genschema` escreve TS a partir do registry Go). Aqui é preciso **avaliar** TypeScript, porque o que `z.object({...})` declara não é legível como texto. O catálogo tem **56 componentes**: 36 do baseline `@json-render/shadcn/catalog` mais 20 do AOS.

- [ ] **Step 1: Install the two dev dependencies**

```bash
cd frontend && npm install --save-dev tsx zod-to-json-schema
```

`frontend/.npmrc` já traz `legacy-peer-deps=true` — não passe a flag na mão, foi exatamente o que abriu a lacuna do `wails3 dev` da fase anterior.

- [ ] **Step 2: Write the generator**

Crie `frontend/scripts/gen-components.mjs`:

```js
// Generates internal/domain/view/components.json from the React component
// catalog.
//
// This is the only generator in the project that runs from TypeScript to Go.
// The other three are Go programs: gencatalog reads Go, gentokens reads CSS as
// text, genschema writes TypeScript from the Go registry. This one has to
// *evaluate* TypeScript, because what `z.object({...})` declares is not
// readable as text — which is why it lives here, in Node, rather than in
// tools/.
//
// Run through tsx: `npx tsx frontend/scripts/gen-components.mjs`.
import { writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { zodToJsonSchema } from "zod-to-json-schema";

import { catalogDefinitions } from "../src/features/view/presentation/components/registry/definitions/catalog.definitions.ts";

const here = dirname(fileURLToPath(import.meta.url));
const out = resolve(here, "../../internal/domain/view/components.json");

// category is what the agent filters by when it composes a screen. The
// definitions do not carry one, so it is derived from the name the same way a
// person would group them — and an unknown component lands in "other" rather
// than being dropped, because a component missing from the catalog is a
// component the agent cannot use.
const CATEGORY = {
  layout: ["Stack", "Grid", "Box", "Card", "Separator", "ScrollArea", "SplitPageLayout",
    "SplitPageSidebar", "SplitPageSidebarHeader", "SplitPageSidebarContent",
    "SplitPageSidebarItem", "SplitPageContent", "SplitPageContentHeader",
    "SplitPageContentBody", "DetailSection"],
  data: ["Table", "Stat", "Progress", "Badge", "Avatar", "Image", "MarkdownContent",
    "DiffStats", "DiffView", "ActivityItem", "ActivityList", "Pagination"],
  input: ["Input", "Textarea", "Select", "Checkbox", "Radio", "Switch", "Slider",
    "Button", "Toggle", "ToggleGroup", "ButtonGroup", "SearchInput", "Link",
    "DropdownMenu"],
  feedback: ["Alert", "Skeleton", "Spinner", "Tooltip", "Popover", "Dialog", "Drawer"],
  navigation: ["Tabs", "TabsSubtle", "Accordion", "Collapsible", "Carousel"],
  typography: ["Heading", "Text", "Icon"],
};

function categoryOf(name) {
  for (const [category, names] of Object.entries(CATEGORY)) {
    if (names.includes(name)) return category;
  }
  return "other";
}

const specs = [];
const failures = [];

for (const [name, def] of Object.entries(catalogDefinitions)) {
  let props = {};
  if (def?.props) {
    try {
      props = zodToJsonSchema(def.props, { target: "jsonSchema7", $refStrategy: "none" });
    } catch (err) {
      // A component whose schema cannot be converted is worse present than
      // absent: present and permissive means the Go side validates nothing and
      // the agent finds out on a blank screen. Fail loud, naming it.
      failures.push(`${name}: ${err.message}`);
      continue;
    }
  }
  const slots = Array.isArray(def?.slots) ? def.slots : [];
  specs.push({
    name,
    description: typeof def?.description === "string" ? def.description : "",
    category: categoryOf(name),
    props,
    slots,
    acceptsChildren: slots.length > 0,
  });
}

if (failures.length > 0) {
  console.error("gen-components: could not convert these component schemas:");
  for (const f of failures) console.error("  " + f);
  process.exit(1);
}

specs.sort((a, b) => a.name.localeCompare(b.name));
writeFileSync(out, JSON.stringify(specs, null, 2) + "\n");
console.log(`gen-components: ${specs.length} components → ${out}`);
```

- [ ] **Step 3: Run the generator and look at what came out**

```bash
cd frontend && npx tsx scripts/gen-components.mjs
```
Expected: `gen-components: 56 components → .../internal/domain/view/components.json`

Abra o JSON e confirme que `Card` tem `props.properties.title`, e que `SplitPageLayout` está lá. Se o número não for 56, o import do catálogo pegou só o baseline do shadcn — confira o caminho.

- [ ] **Step 4: Write the failing Go test**

Crie `internal/domain/view/catalog_test.go`:

```go
package view_test

import (
	"testing"

	"github.com/OWNER/aos/internal/domain/view"
)

// The catalog is the contract between the Go domain and the React design
// system. The original keeps its registry in the frontend and the backend does
// not know it, which is what lets an agent reference a component that does not
// exist; here the catalog is generated and a drift fails the gate.
func TestTheCatalogCarriesTheDesignSystem(t *testing.T) {
	specs := view.Catalog()
	if len(specs) < 50 {
		t.Fatalf("the catalog has %d components, which is too few to be the real one", len(specs))
	}

	for _, name := range []string{"Card", "Stack", "Table", "Button", "SplitPageLayout"} {
		spec, ok := view.LookupComponent(name)
		if !ok {
			t.Fatalf("%s is missing from the catalog", name)
		}
		if spec.Description == "" {
			t.Fatalf("%s has no description; the agent reads it to choose", name)
		}
	}
}

// A component that accepts children is what a tree can nest under. Getting this
// wrong means a valid view is refused, or an invalid one is written.
func TestAContainerAcceptsChildrenAndALeafDoesNot(t *testing.T) {
	stack, ok := view.LookupComponent("Stack")
	if !ok {
		t.Fatal("Stack is missing")
	}
	if !stack.AcceptsChildren {
		t.Fatal("Stack does not accept children, but it is the layout primitive")
	}
}

func TestAnUnknownComponentIsNotFound(t *testing.T) {
	if _, ok := view.LookupComponent("ThisWasNeverBuilt"); ok {
		t.Fatal("the catalog answered for a component that does not exist")
	}
}

// The catalog is sorted so that a regeneration with no change produces no diff.
func TestTheCatalogIsSorted(t *testing.T) {
	specs := view.Catalog()
	for i := 1; i < len(specs); i++ {
		if specs[i-1].Name > specs[i].Name {
			t.Fatalf("%q comes after %q", specs[i-1].Name, specs[i].Name)
		}
	}
}
```

- [ ] **Step 5: Run it to verify it fails**

Run: `go test ./internal/domain/view/ -v`
Expected: FAIL — o pacote `view` ainda não existe.

- [ ] **Step 6: Write the Go side**

Crie `internal/domain/view/catalog.go`:

```go
// Package view is the declarative interface layer: screens described as data,
// validated against the design system's own catalog, and rendered by the
// frontend without a build or a deploy.
package view

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

// componentsJSON is generated from the React catalog by
// frontend/scripts/gen-components.mjs and regenerated by `task gen-components`.
// `task check` fails if it drifts from what is committed, which is what stops a
// component removed in React from staying reachable here.
//
//go:embed components.json
var componentsJSON []byte

// ComponentSpec is the contract between this domain and the React design
// system: what a component is called, what it is for, and what its props are.
//
// Props is JSON Schema rather than a Go struct because the source is a zod
// schema written next to the component. Converting it to a bespoke Go shape
// would be a second description of the same thing, free to disagree with the
// first.
type ComponentSpec struct {
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Category        string         `json:"category"` // layout | data | input | feedback | navigation | typography | other
	Props           map[string]any `json:"props"`
	Slots           []string       `json:"slots"`
	AcceptsChildren bool           `json:"acceptsChildren"`
}

var (
	catalogOnce sync.Once
	catalog     []ComponentSpec
	catalogByID map[string]ComponentSpec
)

func loadCatalog() {
	if err := json.Unmarshal(componentsJSON, &catalog); err != nil {
		// The file is embedded: if it does not parse, it did not parse when the
		// binary was built, and starting anyway would hide a broken build.
		panic(fmt.Sprintf("view: components.json does not parse: %v", err))
	}
	catalogByID = make(map[string]ComponentSpec, len(catalog))
	for _, s := range catalog {
		catalogByID[s.Name] = s
	}
}

// Catalog returns every component the design system publishes, sorted by name.
func Catalog() []ComponentSpec {
	catalogOnce.Do(loadCatalog)
	out := make([]ComponentSpec, len(catalog))
	copy(out, catalog)
	return out
}

// LookupComponent finds one component by name.
func LookupComponent(name string) (ComponentSpec, bool) {
	catalogOnce.Do(loadCatalog)
	s, ok := catalogByID[name]
	return s, ok
}
```

- [ ] **Step 7: Run the test**

Run: `go test ./internal/domain/view/ -v`
Expected: PASS, quatro testes.

- [ ] **Step 8: Wire the generator into the task graph**

Em `Taskfile.yml`, acrescente a task:

```yaml
  gen-components:
    desc: Rebuild the component catalog the view domain validates against
    dir: frontend
    # The only generator that runs from TypeScript to Go: what a zod schema
    # declares is not readable as text, so it has to be evaluated. See the
    # script's own comment.
    cmds: [npx tsx scripts/gen-components.mjs]
```

Acrescente `- task: gen-components` ao fim da lista de `gen`, e a guarda de drift a `check`, logo abaixo da que já existe para o catálogo de erros:

```yaml
      - git diff --exit-code -- internal/domain/view/components.json
```

- [ ] **Step 9: Prove the drift guard works**

```bash
task gen-components && git diff --exit-code -- internal/domain/view/components.json
```
Expected: sem saída, exit 0 — gerar duas vezes dá o mesmo arquivo.

Agora quebre de propósito: apague um componente do `catalog.definitions.ts`, rode `task gen-components`, e confirme que `git diff --exit-code` falha. Depois desfaça.

- [ ] **Step 10: Commit**

```bash
git add frontend/scripts/gen-components.mjs frontend/package.json frontend/package-lock.json \
        internal/domain/view/components.json internal/domain/view/catalog.go \
        internal/domain/view/catalog_test.go Taskfile.yml
git commit -m "feat(view): generate the component catalog from the React design system"
```

---

### Task 4: O domínio `collection` — a definição

**Files:**
- Create: `internal/domain/collection/entity.go`
- Create: `internal/domain/collection/schema.go`
- Create: `internal/domain/collection/errors.go`
- Create: `internal/domain/collection/port.go`
- Create: `internal/domain/collection/validate.go`
- Create: `internal/domain/collection/hooks.go`
- Create: `internal/domain/collection/service.go`
- Create: `internal/domain/collection/collection_test.go`
- Modify: `internal/core/collections/registry.go` (o nativo `collections`)

**Interfaces:**
- Consumes: `collections.Registry` (Task 2).
- Produces:
  - `collection.Collection` com `ID string`, `Name string`, `Description string`, `Scope Scope`, `Skill string`, `Format Format`, `Fields []Field`, `Hooks []Hook`, `CreatedAt/UpdatedAt time.Time`.
  - `collection.Field` com `Name string`, `Type FieldType`, `Description string`, `Required bool`, `Enum []string`, `Ref string`, `Default any`, `Unique bool`.
  - `collection.Validate(c Collection, data map[string]any, existing []map[string]any) error`
  - `collection.ApplyHooks(c Collection, data map[string]any, now time.Time, op string) (map[string]any, error)`
  - `collection.NewService(Deps) *Service` com `List`, `Get`, `Create`, `Delete`.
  - `collection.DescriptorFor(c Collection) (collections.Descriptor, error)` — usada pelas Tasks 5 e 6.
- Task 5 consome `DescriptorFor` e o `Service`; Task 7 consome `Collection.Fields` para validar `bind`.

- [ ] **Step 1: Add the native descriptor for collection definitions**

Em `internal/core/collections/registry.go`, dentro de `var natives`:

```go
	// A collection definition is JSON rather than Markdown because it is
	// schema, not prose — there is no body to write, and every field of it is
	// structured. The records it describes may be either; that is the
	// collection's own Format.
	d("collections", FormatJSON, true,
		Root+"/collections/{id}/schema.json",
		Root+"/skills/{skill}/collections/{id}/schema.json",
	),
	d("views", FormatJSON, false,
		Root+"/views/{id}.view.json",
		Root+"/skills/{skill}/views/{id}.view.json",
	),
	d("toolsets", FormatMarkdown, false,
		Root+"/toolsets/{id}.toolset.md",
		Root+"/skills/{skill}/toolsets/{id}.toolset.md",
	),
```

Rode `go test ./internal/core/collections/` — o teste `TestARegisteredNameMayNotShadowANative` da Task 2 agora protege três nomes a mais, de graça.

- [ ] **Step 2: Write the failing validation test**

Crie `internal/domain/collection/collection_test.go` com a suíte de validação. Este é o coração do domínio e o que a nota `Collection (Go)` pede por inteiro:

```go
package collection_test

import (
	"errors"
	"testing"
	"time"

	"github.com/OWNER/aos/internal/core/apperr"
	"github.com/OWNER/aos/internal/domain/collection"
)

func crm() collection.Collection {
	return collection.Collection{
		ID:     "contacts",
		Name:   "Contacts",
		Scope:  collection.ScopeWorkspace,
		Format: collection.FormatMarkdown,
		Fields: []collection.Field{
			{Name: "name", Type: collection.TypeString, Required: true},
			{Name: "email", Type: collection.TypeString, Unique: true},
			{Name: "stage", Type: collection.TypeEnum, Enum: []string{"lead", "won", "lost"}},
			{Name: "score", Type: collection.TypeNumber},
			{Name: "active", Type: collection.TypeBoolean},
			{Name: "closedAt", Type: collection.TypeDate},
			{Name: "tags", Type: collection.TypeList},
			{Name: "owner", Type: collection.TypeRef, Ref: "agents"},
		},
	}
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T, not an apperr: %v", err, err)
	}
	return app.Code
}

func TestAValidRecordPasses(t *testing.T) {
	err := collection.Validate(crm(), map[string]any{
		"name": "Ada", "email": "ada@example.com", "stage": "won",
		"score": 42.0, "active": true, "tags": []any{"vip"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAMissingRequiredFieldIsRefusedNamingIt(t *testing.T) {
	err := collection.Validate(crm(), map[string]any{"email": "a@b.c"}, nil)
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_REQUIRED" {
		t.Fatalf("code = %q", code)
	}
	var app *apperr.Error
	_ = errors.As(err, &app)
	if app.Issues["field"] != "name" {
		t.Fatalf("the error does not name the field: %v", app.Issues)
	}
}

func TestAValueOutsideTheEnumIsRefused(t *testing.T) {
	err := collection.Validate(crm(), map[string]any{"name": "Ada", "stage": "maybe"}, nil)
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_NOT_IN_ENUM" {
		t.Fatalf("code = %q", code)
	}
}

func TestAValueOfTheWrongTypeIsRefused(t *testing.T) {
	for name, data := range map[string]map[string]any{
		"string given a number": {"name": 7},
		"number given a string": {"name": "Ada", "score": "muito"},
		"boolean given a word":  {"name": "Ada", "active": "sim"},
		"date given nonsense":   {"name": "Ada", "closedAt": "ontem"},
		"list given a scalar":   {"name": "Ada", "tags": "vip"},
	} {
		err := collection.Validate(crm(), data, nil)
		if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_WRONG_TYPE" {
			t.Fatalf("%s: code = %q", name, code)
		}
	}
}

// Unique is checked against what is already stored. Without the existing
// records there is nothing to compare against, which is why they are a
// parameter rather than something Validate goes and fetches — the domain does
// not do IO.
func TestADuplicateUniqueValueIsRefused(t *testing.T) {
	existing := []map[string]any{{"name": "Ada", "email": "ada@example.com"}}
	err := collection.Validate(crm(), map[string]any{"name": "Grace", "email": "ada@example.com"}, existing)
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_NOT_UNIQUE" {
		t.Fatalf("code = %q", code)
	}
}

func TestAFieldNotDeclaredIsRefused(t *testing.T) {
	err := collection.Validate(crm(), map[string]any{"name": "Ada", "salary": 100}, nil)
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_UNDECLARED" {
		t.Fatalf("code = %q", code)
	}
}

// A ref pointing at a collection that does not exist cannot render: the view
// has nothing to resolve the relation against. Refusing at declaration time is
// cheaper than refusing at render time.
func TestARefToAnUnknownCollectionIsRefusedAtDeclaration(t *testing.T) {
	c := crm()
	c.Fields = append(c.Fields, collection.Field{Name: "deal", Type: collection.TypeRef, Ref: "does-not-exist"})
	err := collection.ValidateSchema(c, func(name string) bool { return name == "agents" })
	if code := codeOf(t, err); code != "AOS_COLLECTION_REF_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}

func TestARefFieldWithoutARefTargetIsRefused(t *testing.T) {
	c := crm()
	c.Fields = []collection.Field{{Name: "owner", Type: collection.TypeRef}}
	err := collection.ValidateSchema(c, func(string) bool { return true })
	if code := codeOf(t, err); code != "AOS_COLLECTION_REF_MISSING" {
		t.Fatalf("code = %q", code)
	}
}

// Hooks are declarative — a small set of named actions with parameters, never
// source code. The original accepts code strings and writes them into a
// generated schema.ts, which is an agent producing executable code in the
// workspace with no sandbox and no review.
func TestDeclarativeHooksNormaliseAndStamp(t *testing.T) {
	c := crm()
	c.Fields = append(c.Fields, collection.Field{Name: "slug", Type: collection.TypeString},
		collection.Field{Name: "createdAt", Type: collection.TypeDate})
	c.Hooks = []collection.Hook{
		{On: "create", Action: collection.ActionSetTimestamp, Field: "createdAt"},
		{On: "create", Action: collection.ActionSlugify, Field: "slug", From: "name"},
		{On: "create", Action: collection.ActionDefaultTo, Field: "stage", Value: "lead"},
	}
	at := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	out, err := collection.ApplyHooks(c, map[string]any{"name": "Ada Lovelace"}, at, "create")
	if err != nil {
		t.Fatal(err)
	}
	if out["slug"] != "ada-lovelace" {
		t.Fatalf("slug = %v", out["slug"])
	}
	if out["stage"] != "lead" {
		t.Fatalf("stage = %v", out["stage"])
	}
	if out["createdAt"] != at.Format(time.RFC3339) {
		t.Fatalf("createdAt = %v", out["createdAt"])
	}
}

// defaultTo must not overwrite a value somebody supplied — that is the whole
// difference between a default and an assignment.
func TestDefaultToDoesNotOverwriteAGivenValue(t *testing.T) {
	c := crm()
	c.Hooks = []collection.Hook{{On: "create", Action: collection.ActionDefaultTo, Field: "stage", Value: "lead"}}
	out, err := collection.ApplyHooks(c, map[string]any{"name": "Ada", "stage": "won"}, time.Unix(0, 0), "create")
	if err != nil {
		t.Fatal(err)
	}
	if out["stage"] != "won" {
		t.Fatalf("stage = %v, want the supplied value kept", out["stage"])
	}
}

// A hook naming an action this build does not implement is a refusal, not a
// silent skip: a normalisation that quietly did not happen is a record that
// looks right and is not.
func TestAnUnknownHookActionIsRefused(t *testing.T) {
	c := crm()
	c.Hooks = []collection.Hook{{On: "create", Action: "runShellCommand", Field: "name"}}
	_, err := collection.ApplyHooks(c, map[string]any{"name": "Ada"}, time.Unix(0, 0), "create")
	if code := codeOf(t, err); code != "AOS_COLLECTION_HOOK_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./internal/domain/collection/ -v`
Expected: FAIL — o pacote não existe.

- [ ] **Step 4: Write the entity and the schema types**

`internal/domain/collection/entity.go`:

```go
// Package collection is structured domain data defined at runtime by the
// agent. It is what lets "build me a CRM" become real tables without a
// programmer.
//
// The schema is data and is never evaluated. That is not a performance
// decision: it is why loading a schema an agent wrote is safe.
package collection

import "time"

// Scope says whether a collection belongs to the workspace or came with a
// skill. A skill-scoped collection is removed when the skill is uninstalled.
type Scope string

const (
	ScopeWorkspace Scope = "workspace"
	ScopeSkill     Scope = "skill"
)

// Format is how each record of this collection is laid out on disk.
type Format string

const (
	FormatMarkdown Format = "md"
	FormatJSON     Format = "json"
)

// FieldType is the constrained subset of JSON Schema this engine understands:
// enough to describe records, not enough to be a programming language.
type FieldType string

const (
	TypeString  FieldType = "string"
	TypeNumber  FieldType = "number"
	TypeBoolean FieldType = "boolean"
	TypeDate    FieldType = "date"
	TypeEnum    FieldType = "enum"
	TypeRef     FieldType = "ref"
	TypeList    FieldType = "list"
)

// Field is one declared column.
type Field struct {
	Name        string    `json:"name"`
	Type        FieldType `json:"type"`
	Description string    `json:"description,omitempty"`
	Required    bool      `json:"required,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
	Ref         string    `json:"ref,omitempty"` // another collection's id
	Default     any       `json:"default,omitempty"`
	Unique      bool      `json:"unique,omitempty"`
}

// Collection is the declaration. The records it describes live under it and
// are collections.Record values, not Go structs.
type Collection struct {
	ID string `json:"id" collection:"path"`

	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Scope       Scope  `json:"scope"`
	Skill       string `json:"skill,omitempty" collection:"path=skill"`
	Format      Format `json:"format"`

	Fields []Field `json:"fields"`
	Hooks  []Hook  `json:"hooks,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
```

`internal/domain/collection/hooks.go`:

```go
package collection

import (
	"strings"
	"time"
	"unicode"
)

// HookAction is the closed set of normalisations a collection may declare.
//
// The original accepts source-code strings for onCreated/onUpdated/onDeleted
// and writes them into a generated schema.ts — an agent producing executable
// code in the workspace, with no sandbox and no review. These cover the cases
// actually observed there (timestamps and normalisation) without opening that
// door. Anything beyond them is a Routine with an activity trigger, which goes
// through the sandbox and is audited.
type HookAction string

const (
	ActionSetTimestamp HookAction = "setTimestamp"
	ActionSlugify      HookAction = "slugify"
	ActionDefaultTo    HookAction = "defaultTo"
	ActionComputeFrom  HookAction = "computeFrom"
)

// Hook is one declared normalisation.
type Hook struct {
	On     string     `json:"on"` // create | update
	Action HookAction `json:"action"`
	Field  string     `json:"field"`
	From   string     `json:"from,omitempty"`  // slugify, computeFrom
	Value  any        `json:"value,omitempty"` // defaultTo
}

// ApplyHooks runs the collection's declared hooks over a record. It returns a
// new map rather than mutating: the caller still holds what the agent sent, and
// an error halfway through must not leave a half-normalised record behind.
func ApplyHooks(c Collection, data map[string]any, now time.Time, op string) (map[string]any, error) {
	out := make(map[string]any, len(data)+len(c.Hooks))
	for k, v := range data {
		out[k] = v
	}
	for _, h := range c.Hooks {
		if h.On != "" && h.On != op {
			continue
		}
		switch h.Action {
		case ActionSetTimestamp:
			out[h.Field] = now.UTC().Format(time.RFC3339)
		case ActionSlugify:
			source, _ := out[h.From].(string)
			out[h.Field] = slugify(source)
		case ActionDefaultTo:
			if existing, ok := out[h.Field]; !ok || existing == nil || existing == "" {
				out[h.Field] = h.Value
			}
		case ActionComputeFrom:
			if v, ok := out[h.From]; ok {
				out[h.Field] = v
			}
		default:
			return nil, errHookUnknown(c.ID, string(h.Action))
		}
	}
	return out, nil
}

// slugify is deliberately small: lowercase, letters and digits kept, everything
// else collapsed to a single hyphen. A slug is a path segment, and a path
// segment with a surprise in it is a path traversal waiting to be found.
func slugify(s string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteRune('-')
			lastHyphen = true
		}
	}
	return strings.Trim(b.String(), "-")
}
```

- [ ] **Step 5: Write validate.go**

`internal/domain/collection/validate.go`. Duas funções: `ValidateSchema` (a declaração faz sentido) e `Validate` (um registro cabe na declaração).

```go
package collection

import (
	"fmt"
	"time"
)

// Exists reports whether a collection name is known. It is a parameter rather
// than a registry this package holds, because internal/domain does no lookups
// of its own — the caller in the service knows the registry.
type Exists func(name string) bool

// ValidateSchema checks that a declaration is coherent before it is stored: a
// ref has a target, the target exists, an enum has values, and no field is
// declared twice.
func ValidateSchema(c Collection, exists Exists) error {
	if len(c.Fields) == 0 {
		return errNoFields(c.ID)
	}
	seen := map[string]bool{}
	for _, f := range c.Fields {
		if f.Name == "" {
			return errFieldUnnamed(c.ID)
		}
		if seen[f.Name] {
			return errFieldDuplicated(c.ID, f.Name)
		}
		seen[f.Name] = true

		switch f.Type {
		case TypeString, TypeNumber, TypeBoolean, TypeDate, TypeList:
		case TypeEnum:
			if len(f.Enum) == 0 {
				return errEnumEmpty(c.ID, f.Name)
			}
		case TypeRef:
			if f.Ref == "" {
				return errRefMissing(c.ID, f.Name)
			}
			if exists != nil && !exists(f.Ref) {
				return errRefUnknown(c.ID, f.Name, f.Ref)
			}
		default:
			return errFieldTypeUnknown(c.ID, f.Name, string(f.Type))
		}
	}
	return nil
}

// Validate checks a record against the declared fields.
//
// existing is what is already stored, and it is a parameter for the same reason
// Exists is: the domain does not read the disk. Pass nil when there is nothing
// to compare against — uniqueness then has nothing to violate.
func Validate(c Collection, data map[string]any, existing []map[string]any) error {
	byName := make(map[string]Field, len(c.Fields))
	for _, f := range c.Fields {
		byName[f.Name] = f
	}

	// An undeclared field is refused rather than dropped. Dropping it would
	// mean an agent writes a record, reads it back, and finds part of what it
	// wrote missing with nothing having said so.
	for name := range data {
		if _, ok := byName[name]; !ok {
			return errFieldUndeclared(c.ID, name)
		}
	}

	for _, f := range c.Fields {
		raw, present := data[f.Name]
		if !present || raw == nil {
			if f.Required {
				return errFieldRequired(c.ID, f.Name)
			}
			continue
		}
		if err := checkType(c.ID, f, raw); err != nil {
			return err
		}
		if f.Unique {
			for _, other := range existing {
				if fmt.Sprint(other[f.Name]) == fmt.Sprint(raw) {
					return errFieldNotUnique(c.ID, f.Name, raw)
				}
			}
		}
	}
	return nil
}

func checkType(id string, f Field, raw any) error {
	switch f.Type {
	case TypeString:
		if _, ok := raw.(string); !ok {
			return errFieldWrongType(id, f.Name, "string", raw)
		}
	case TypeNumber:
		switch raw.(type) {
		case float64, float32, int, int64:
		default:
			return errFieldWrongType(id, f.Name, "number", raw)
		}
	case TypeBoolean:
		if _, ok := raw.(bool); !ok {
			return errFieldWrongType(id, f.Name, "boolean", raw)
		}
	case TypeDate:
		s, ok := raw.(string)
		if !ok {
			return errFieldWrongType(id, f.Name, "date", raw)
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return errFieldWrongType(id, f.Name, "date in RFC 3339", raw)
		}
	case TypeEnum:
		s, ok := raw.(string)
		if !ok {
			return errFieldWrongType(id, f.Name, "string", raw)
		}
		for _, allowed := range f.Enum {
			if s == allowed {
				return nil
			}
		}
		return errFieldNotInEnum(id, f.Name, s, f.Enum)
	case TypeRef:
		if _, ok := raw.(string); !ok {
			return errFieldWrongType(id, f.Name, "the id of a record, as a string", raw)
		}
	case TypeList:
		if _, ok := raw.([]any); !ok {
			return errFieldWrongType(id, f.Name, "list", raw)
		}
	}
	return nil
}
```

- [ ] **Step 6: Write errors.go**

Todos os erros usados acima, no formato do projeto. Cada um com `.Causer()`, `.Msgf()`, `.Issue()` nomeando o campo, `.Status()` e `.CTA()`. Exemplo do padrão, e os outros seguem-no:

```go
package collection

import "github.com/OWNER/aos/internal/core/apperr"

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
		CTA(apperr.CallToAction{Label: "send the field as " + want})
}
```

Os quinze, todos com prefixo `COLLECTION_`, com o status e o que a CTA precisa dizer. O status não é decoração: `httpapi` o devolve, e a UI decide por ele se mostra "corrija isto" ou "algo quebrou".

| Código | Causer | Status | O que a CTA tem que dizer |
|---|---|---|---|
| `FIELD_REQUIRED` | `collection.Validate` | 400 | inclua o campo, ou leia o schema da coleção |
| `FIELD_WRONG_TYPE` | `collection.Validate` | 400 | envie o campo como o tipo declarado, nomeando-o |
| `FIELD_NOT_IN_ENUM` | `collection.Validate` | 400 | use um dos valores declarados, e liste-os em `Issue("allowed", …)` |
| `FIELD_NOT_UNIQUE` | `collection.Validate` | 409 | outro registro já tem esse valor; procure-o antes de criar |
| `FIELD_UNDECLARED` | `collection.Validate` | 400 | o campo não existe no schema; declare-o ou remova-o do registro |
| `FIELD_UNNAMED` | `collection.ValidateSchema` | 400 | todo campo precisa de nome, que é como um registro se refere a ele |
| `FIELD_DUPLICATED` | `collection.ValidateSchema` | 400 | o campo foi declarado duas vezes; o segundo esconderia o primeiro |
| `FIELD_TYPE_UNKNOWN` | `collection.ValidateSchema` | 400 | liste os sete tipos aceitos em `Issue("allowed", …)` |
| `ENUM_EMPTY` | `collection.ValidateSchema` | 400 | um enum sem valores não aceita nada; declare os valores |
| `REF_MISSING` | `collection.ValidateSchema` | 400 | um campo `ref` precisa dizer para qual coleção aponta |
| `REF_UNKNOWN` | `collection.ValidateSchema` | 400 | crie a coleção referenciada antes, ou corrija o nome |
| `NO_FIELDS` | `collection.ValidateSchema` | 400 | uma coleção sem campos não descreve registro nenhum |
| `HOOK_UNKNOWN` | `collection.ApplyHooks` | 400 | liste as quatro ações declarativas; código-fonte não é uma delas, e o caminho para lógica além disso é uma Routine com trigger `activity` |
| `NOT_FOUND` | `collection.Service.Get` | 404 | liste as coleções que existem antes de escrever nesta |
| `NAME_INVALID` | `collection.DescriptorFor` | 400 | o id é um segmento de caminho: minúsculas, dígitos, hífen e sublinhado |

Nenhum deles é 500. Todos descrevem algo que quem chamou pode corrigir — e um 500 para entrada malformada manda a pessoa procurar um defeito que não existe.

- [ ] **Step 7: Run the validation suite**

Run: `go test ./internal/domain/collection/ -v`
Expected: PASS, todos os testes do Step 2.

- [ ] **Step 8: Write port.go, DescriptorFor and the service**

`internal/domain/collection/port.go`:

```go
package collection

import (
	"context"
	"time"

	"github.com/OWNER/aos/internal/core/collections"
)

// Repository persists collection declarations. It is the engine's repository
// bound to the "collections" native.
type Repository interface {
	Get(ctx context.Context, key collections.Key) (*Collection, error)
	List(ctx context.Context, q collections.Query) ([]Collection, error)
	Create(ctx context.Context, v *Collection) error
	Update(ctx context.Context, v *Collection, expect collections.Version) error
	Delete(ctx context.Context, key collections.Key) error
}

// Clock is the only source of time in this package.
type Clock interface{ Now() time.Time }
```

`internal/domain/collection/schema.go` — `DescriptorFor`, que é o ponto onde uma declaração vira algo que o motor entende:

```go
package collection

import (
	"path"

	"github.com/OWNER/aos/internal/core/collections"
)

// DescriptorFor turns a declaration into the engine descriptor its records are
// stored under.
//
// The collection's id is baked into the pattern rather than left as a
// placeholder: a collection named "contacts" becomes a descriptor named
// "contacts" whose records live at a fixed path. That is what keeps Query.Key
// meaning what it means everywhere else — {id} identifies a record, not which
// collection it belongs to.
//
// A skill-scoped collection gets a second, read-only pattern, by the same rule
// the natives already use: a pattern with a wildcard is not writable.
func DescriptorFor(c Collection) (collections.Descriptor, error) {
	if err := validName(c.ID); err != nil {
		return collections.Descriptor{}, err
	}
	ext := "md"
	format := collections.FormatMarkdown
	if c.Format == FormatJSON {
		ext = "json"
		format = collections.FormatJSON
	}

	patterns := []*collections.Pattern{
		collections.MustCompile(path.Join(collections.Root, "collections", c.ID, "records", "{id}."+ext)),
	}
	if c.Scope == ScopeSkill {
		patterns = append(patterns, collections.MustCompile(
			path.Join(collections.Root, "skills", "*", "collections", c.ID, "records", "{id}."+ext)))
	}
	return collections.Descriptor{
		Name:          c.ID,
		Patterns:      patterns,
		Format:        format,
		CascadeDelete: false,
	}, nil
}

// validName keeps an id usable as a path segment. It is the same reasoning as
// slugify's: an id is a directory name, and a directory name with a separator
// or a dot-dot in it is a path traversal.
func validName(id string) error {
	if id == "" || id == "." || id == ".." {
		return errNameInvalid(id)
	}
	for _, r := range id {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return errNameInvalid(id)
		}
	}
	return nil
}
```

`internal/domain/collection/service.go` com `Deps{Repo Repository, Registry *collections.Registry, Clock Clock}`, e `List`, `Get`, `Create`, `Delete`. `Create` faz, nesta ordem: `validName` → `ValidateSchema` com `exists` fechando sobre `Registry.Lookup` → `DescriptorFor` → `Repo.Create` → `Registry.Register`. **Registrar por último**, pela mesma razão da instalação de skill: uma falha de escrita não pode deixar um nome registrado sem arquivo por trás.

`Delete` inverte: `Registry.Unregister` primeiro, depois `Repo.Delete` — nada deve conseguir escrever num registro cuja declaração está a caminho de sumir.

- [ ] **Step 9: Test the service against a fake repository**

Acrescente ao arquivo de teste um `fakeRepo` em memória (o domínio não toca IO) e prove:

```go
// A collection is registered only after it is stored. A name that resolved to
// nothing on disk would be a collection you can write to and never read back.
func TestCreateRegistersOnlyAfterItPersisted(t *testing.T) {
	repo := &fakeRepo{failCreate: errors.New("the disk is full")}
	reg := collections.NewRegistry()
	svc := collection.NewService(collection.Deps{Repo: repo, Registry: reg, Clock: clockx.Fixed{At: at}})

	_, err := svc.Create(ctx(), collection.CreateInput{
		ID: "contacts", Name: "Contacts", Format: collection.FormatMarkdown,
		Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
	})
	if err == nil {
		t.Fatal("a failing write reported success")
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the collection was registered even though nothing was stored")
	}
}

// Deleting unregisters before removing, so nothing can write a record whose
// declaration is on its way out.
func TestDeleteUnregistersBeforeRemoving(t *testing.T) {
	var order []string
	reg := collections.NewRegistry()
	repo := &fakeRepo{onDelete: func() { order = append(order, "removed") }}
	svc := collection.NewService(collection.Deps{Repo: repo, Registry: reg, Clock: clockx.Fixed{At: at}})

	mustCreate(t, svc, "contacts")
	repo.onUnregisterObserved = func() { order = append(order, "unregistered") }

	if err := svc.Delete(ctx(), collection.DeleteInput{ID: "contacts"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the collection is still registered after being deleted")
	}
	// Nothing may write a record whose declaration is on its way out.
	if len(order) != 2 || order[0] != "unregistered" {
		t.Fatalf("order = %v, want unregister before remove", order)
	}
}

// A reserved name is refused by the registry, and the refusal reaches the
// caller unchanged rather than being restated here.
func TestCreatingACollectionNamedAgentsIsRefused(t *testing.T) {
	svc := collection.NewService(collection.Deps{
		Repo: &fakeRepo{}, Registry: collections.NewRegistry(), Clock: clockx.Fixed{At: at},
	})
	_, err := svc.Create(ctx(), collection.CreateInput{
		ID: "agents", Name: "Agents", Format: collection.FormatMarkdown,
		Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
	})
	// The registry refuses it, and the refusal reaches the caller unchanged
	// rather than being restated here — one list of reserved names, in one
	// place.
	if code := codeOf(t, err); code != "AOS_COLLECTION_NAME_RESERVED" {
		t.Fatalf("code = %q", code)
	}
}

// An id that is not a safe path segment is refused before anything touches the
// filesystem.
func TestAnIdThatIsNotAPathSegmentIsRefused(t *testing.T) {
	svc := collection.NewService(collection.Deps{
		Repo: &fakeRepo{}, Registry: collections.NewRegistry(), Clock: clockx.Fixed{At: at},
	})
	// An id is a directory name. One with a separator or a dot-dot in it is a
	// path traversal, and it has to be refused before anything is resolved.
	for _, id := range []string{"", ".", "..", "../escape", "with/slash", "With Caps", "acentuação"} {
		_, err := svc.Create(ctx(), collection.CreateInput{
			ID: id, Name: "x", Format: collection.FormatMarkdown,
			Fields: []collection.Field{{Name: "name", Type: collection.TypeString}},
		})
		if err == nil {
			t.Fatalf("%q was accepted as a collection id", id)
		}
		if code := codeOf(t, err); code != "AOS_COLLECTION_NAME_INVALID" {
			t.Fatalf("%q: code = %q, want AOS_COLLECTION_NAME_INVALID", id, code)
		}
	}
}
```

- [ ] **Step 10: Run everything and commit**

```bash
go test -race ./internal/domain/collection/ ./internal/core/collections/
task vet && task lint
git add internal/domain/collection internal/core/collections/registry.go
git commit -m "feat(collection): declarations, validation, and declarative hooks"
```

---

### Task 5: O domínio `collection` — os registros

**Files:**
- Create: `internal/domain/collection/record.go`
- Modify: `internal/domain/collection/service.go` (`Records() RecordService`)
- Modify: `internal/domain/collection/collection_test.go`

**Interfaces:**
- Consumes: `collections.Record` (Task 1), `*collections.Registry` (Task 2), `collection.Validate`, `collection.ApplyHooks`, `collection.DescriptorFor` (Task 4).
- Produces: `collection.RecordService` com `List(ctx, collectionID string, q RecordQuery) ([]Record, error)`, `Get(ctx, collectionID, id string) (*Record, error)`, `Create(ctx, collectionID string, data map[string]any) (*Record, error)`, `Update(ctx, collectionID, id string, data map[string]any) (*Record, error)`, `Delete(ctx, collectionID, id string) error`. `collection.Record` é `struct{ ID string; Collection string; Data map[string]any; Content string; CreatedAt, UpdatedAt time.Time }`. Tasks 7 e 10 dependem disso.
- Produces também: `collection.RecordRepositories` — o port que devolve um `RecordRepo` para um id de coleção, implementado em `internal/adapters/fscollections` na Task 10.

**Contexto:** o domínio não pode construir um repositório de disco (`TestDependencyRule`). Então `RecordService` recebe uma fábrica como port: dado o id da coleção, devolve o repositório dos registros dela.

- [ ] **Step 1: Write the failing test**

```go
// A record is validated against its collection's declaration before it is
// stored, and normalised by the collection's hooks. Neither is optional: the
// point of declaring a schema is that what is stored matches it.
func TestCreatingARecordValidatesAndNormalises(t *testing.T) {
	c := crm()
	c.Fields = append(c.Fields, collection.Field{Name: "createdAt", Type: collection.TypeDate})
	c.Hooks = []collection.Hook{{On: "create", Action: collection.ActionSetTimestamp, Field: "createdAt"}}
	svc := newRecordService(t, c)

	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada", "stage": "lead"})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Data["createdAt"] != at.Format(time.RFC3339) {
		t.Fatalf("the hook did not run: %v", rec.Data)
	}
	// And the refusal path, from the same service: validation is not optional.
	if _, err := svc.Create(ctx(), "contacts", map[string]any{"stage": "lead"}); err == nil {
		t.Fatal("a record missing a required field was stored")
	}
}

// The unique check needs what is already stored, so the service is what
// fetches it and hands it to Validate — the domain function stays pure.
func TestCreatingASecondRecordWithADuplicateUniqueValueIsRefused(t *testing.T) {
	svc := newRecordService(t, crm())

	if _, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada", "email": "a@b.c"}); err != nil {
		t.Fatal(err)
	}
	// The unique check needs what is already stored, so the service is what
	// fetches it and hands it to Validate — the domain function stays pure.
	_, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Grace", "email": "a@b.c"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_NOT_UNIQUE" {
		t.Fatalf("code = %q", code)
	}
}

// A record in a collection nobody declared is refused, naming it, rather than
// creating the collection implicitly. An implicit collection is a typo that
// becomes a schema.
func TestARecordInAnUnknownCollectionIsRefused(t *testing.T) {
	svc := newRecordService(t, crm())

	// An implicit collection is a typo that becomes a schema.
	_, err := svc.Create(ctx(), "contatos", map[string]any{"name": "Ada"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_NOT_FOUND" {
		t.Fatalf("code = %q", code)
	}
	var app *apperr.Error
	_ = errors.As(err, &app)
	if app.Issues["collection"] != "contatos" {
		t.Fatalf("the error does not name what was asked for: %v", app.Issues)
	}
}

// Update revalidates. A record that was valid when written and invalid after
// an edit is exactly what validation is for.
func TestUpdatingARecordRevalidates(t *testing.T) {
	svc := newRecordService(t, crm())
	rec, err := svc.Create(ctx(), "contacts", map[string]any{"name": "Ada", "stage": "lead"})
	if err != nil {
		t.Fatal(err)
	}

	// A record that was valid when written and invalid after an edit is
	// exactly what validation is for.
	_, err = svc.Update(ctx(), "contacts", rec.ID, map[string]any{"name": "Ada", "stage": "talvez"})
	if code := codeOf(t, err); code != "AOS_COLLECTION_FIELD_NOT_IN_ENUM" {
		t.Fatalf("code = %q", code)
	}
	// And the stored record is untouched: a refused update is not a partial one.
	again, err := svc.Get(ctx(), "contacts", rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Data["stage"] != "lead" {
		t.Fatalf("stage = %v, want the refused update to have changed nothing", again.Data["stage"])
	}
}

// The Markdown body of a record is content, not a field: it round-trips
// without being declared in the schema.
func TestARecordsBodyIsNotAField(t *testing.T) {
	svc := newRecordService(t, crm())

	// The Markdown body is content, not a column: it round-trips without being
	// declared, and declaring every note-taking collection a "content" field
	// would be describing the format twice.
	rec, err := svc.CreateWithContent(ctx(), "contacts",
		map[string]any{"name": "Ada"}, "Conheceu o Babbage numa festa.")
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.Get(ctx(), "contacts", rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Content != "Conheceu o Babbage numa festa." {
		t.Fatalf("content = %q", again.Content)
	}
	if _, isField := again.Data["content"]; isField {
		t.Fatal("the body leaked into the fields")
	}
}
```

Escreva cada um por extenso, contra o `fakeRepo` da Task 4 mais um `fakeRecordRepos` em memória.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/domain/collection/ -run TestCreatingARecord -v`
Expected: FAIL — `RecordService` não existe.

- [ ] **Step 3: Write record.go**

`Record` do domínio, `RecordQuery`, o port `RecordRepositories`, e a implementação de `recordService`. `Create` faz, nesta ordem:

1. `Get` da declaração — se não existe, `errNotFound` nomeando a coleção;
2. `ApplyHooks(c, data, clock.Now(), "create")`;
3. `List` dos existentes **apenas se houver algum campo `Unique`** — não pague uma varredura por coleção que não pede unicidade;
4. `Validate(c, normalised, existing)`;
5. `repo.Create` com um `collections.Record{Key: {"id": id}, Fields: normalised, Content: content}`.

- [ ] **Step 4: Run the tests**

Run: `go test -race ./internal/domain/collection/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/domain/collection
git commit -m "feat(collection): records, validated against the schema they declared"
```

---

### Task 6: O watcher ligado ao bus — autoWatch e o resíduo A2

**Files:**
- Create: `internal/app/collections_watch.go`
- Modify: `internal/app/wire.go`
- Test: `internal/app/collections_watch_test.go`

**Interfaces:**
- Consumes: `*collections.Registry` (Task 2), `collection.DescriptorFor` e `collection.Service` (Task 4), `fscollections.Watcher` e `WithWatchPublisher` (já existem).
- Produces: `app.startCollectionWatch(ctx, deps) (stop func(), err error)`, chamada de `Serve`.

**Contexto e por que isto é uma task só:** `fscollections.WithPublisher` e `WithWatchPublisher` existem desde a Fase 1 e **nunca tiveram caller**. É o resíduo A2 do branch anterior — `files:changed` está mapeado no frontend para `collection.changed`, que nada no Go publica, e por isso a árvore de arquivos não se atualiza sozinha. O `autoWatch` das coleções dinâmicas precisa exatamente do mesmo watcher ligado ao bus. Uma coisa fecha a outra, e separá-las produziria duas fiações do mesmo watcher.

- [ ] **Step 1: Write the failing test**

```go
// A schema.json that appears on disk without this process having written it —
// another aos, or somebody editing the file by hand, which is the entire point
// of ADR-0004 — becomes a usable collection without a restart.
func TestASchemaAppearingOnDiskRegistersTheCollection(t *testing.T) {
	repo := t.TempDir()
	a := newAppAt(t, t.TempDir(), repo, "")
	t.Cleanup(func() { _ = a.Close() })

	if _, ok := a.CollectionRegistry.Lookup("contacts"); ok {
		t.Fatal("contacts existed before the file did")
	}

	// Written by something that is not this process — another aos, or somebody
	// editing the file by hand, which is the entire point of ADR-0004.
	dir := filepath.Join(repo, collections.Root, "collections", "contacts")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	schema := `{"id":"contacts","name":"Contacts","scope":"workspace","format":"md",` +
		`"fields":[{"name":"name","type":"string","required":true}]}`
	if err := os.WriteFile(filepath.Join(dir, "schema.json"), []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}

	waitFor(t, 2*time.Second, func() bool {
		_, ok := a.CollectionRegistry.Lookup("contacts")
		return ok
	})
}

// And a schema removed from disk stops being registered, so a write to it
// fails with "no such collection" rather than landing in a directory nobody
// declares any more.
func TestASchemaRemovedFromDiskUnregistersTheCollection(t *testing.T) {
	repo := t.TempDir()
	a := newAppAt(t, t.TempDir(), repo, "")
	t.Cleanup(func() { _ = a.Close() })

	path := writeSchema(t, repo, "contacts")
	waitFor(t, 2*time.Second, func() bool {
		_, ok := a.CollectionRegistry.Lookup("contacts")
		return ok
	})

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// A write to it must then fail with "no such collection" rather than
	// landing in a directory nobody declares any more.
	waitFor(t, 2*time.Second, func() bool {
		_, ok := a.CollectionRegistry.Lookup("contacts")
		return !ok
	})
}

// Every successful write publishes Changed on the bus. This is what the
// frontend's realtime map is already listening for and has never received.
func TestAWriteToACollectionPublishesChanged(t *testing.T) {
	repo := t.TempDir()
	a := newAppAt(t, t.TempDir(), repo, "")
	t.Cleanup(func() { _ = a.Close() })

	// This is what the frontend's realtime map has been listening for and has
	// never received: WithPublisher existed since phase 1 with no caller.
	seen := make(chan collections.Changed, 4)
	a.Events.Subscribe("collection.changed", func(ev collections.Changed) { seen <- ev })

	writeSchema(t, repo, "contacts")
	waitFor(t, 2*time.Second, func() bool {
		_, ok := a.CollectionRegistry.Lookup("contacts")
		return ok
	})
	if _, err := a.Collections.Records().Create(ctx(), "contacts", map[string]any{"name": "Ada"}); err != nil {
		t.Fatal(err)
	}

	select {
	case ev := <-seen:
		if ev.Collection != "contacts" || ev.Op != "create" {
			t.Fatalf("event = %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a write produced no Changed event; the publisher still has no caller")
	}
}
```

Este teste vive em `internal/app`, contra disco real, porque é fiação e não domínio.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/app/ -run TestASchema -v`
Expected: FAIL.

- [ ] **Step 3: Wire the publisher into every repository**

Em `internal/app/wire.go`, na função que monta o `repoSet`, acrescente `fscollections.WithPublisher[T](events)` a cada `fscollections.New`, ao lado do `WithLock` e do `WithIndex` que já estão lá.

**Cuidado:** `events` é o `*realtime.Hub`. Confira que ele satisfaz `collections.Publisher` (`Publish(ctx, Changed)`); se não satisfizer, escreva o adaptador fino em `internal/app` — não altere a interface do motor para caber no hub.

- [ ] **Step 4: Write the watch loop**

`internal/app/collections_watch.go` constrói o `fscollections.Watcher` sobre a raiz do workspace com `WithWatchPublisher(events)`, e um laço que, para cada evento cujo caminho case com `.aos/collections/{id}/schema.json` ou `.aos/skills/{skill}/collections/{id}/schema.json`:

- criação ou alteração → lê a declaração, `DescriptorFor`, `Registry.Register`;
- remoção → `Registry.Unregister`.

O laço roda numa goroutine com `context`, e `stop` a encerra. Registre falhas com `log.Warn` nomeando o arquivo — um schema ilegível não pode derrubar o daemon, mas também não pode sumir em silêncio.

- [ ] **Step 5: Boot-time scan**

No `New` do app, depois de montar o registry: varra `.aos/collections/*/schema.json` uma vez e registre o que achar. Sem isto, uma coleção criada ontem só existe depois que alguém a tocar.

- [ ] **Step 6: Run the tests, then confirm A2 is closed**

```bash
go test -race ./internal/app/ -run "TestASchema|TestAWrite" -v
```

E empiricamente, não por dedução: suba o daemon, abra o WebSocket, escreva um arquivo, e confirme que `collection.changed` chega. A lição registrada no ledger da fase anterior foi exatamente esta — B1(b) foi dado como corrigido "no nome" e não estava.

- [ ] **Step 7: Commit**

```bash
git add internal/app/collections_watch.go internal/app/collections_watch_test.go internal/app/wire.go
git commit -m "feat(app): the collections watcher gets a caller — autoWatch, and A2 closed"
```

---

### Task 7: O domínio `view`

**Files:**
- Create: `internal/domain/view/entity.go`
- Create: `internal/domain/view/errors.go`
- Create: `internal/domain/view/port.go`
- Create: `internal/domain/view/validate.go`
- Create: `internal/domain/view/scaffold.go`
- Create: `internal/domain/view/service.go`
- Create: `internal/domain/view/view_test.go`

**Interfaces:**
- Consumes: `view.ComponentSpec`, `view.Catalog`, `view.LookupComponent` (Task 3); `collection.Collection`, `collection.Field` (Task 4); `collection.RecordService` (Task 5). O nativo `views` já foi acrescentado na Task 4, Step 1.
- Produces:
  - `view.View` com `ID`, `Name`, `Title`, `Description`, `Scope`, `Skill`, `Source Source`, `Tree Node`, `CreatedAt`, `UpdatedAt`.
  - `view.Node` com `Component string`, `Props map[string]any`, `Bind map[string]string`, `Children []Node`, `Actions []Action`.
  - `view.Action` com `Label string`, `Command string`, `Input map[string]any`, `Confirm bool`.
  - `view.NewService(Deps) *Service` com `List`, `Get`, `Create`, `Delete`, `Render`, `Components`, `Scaffold`, `ExecuteAction`.
- Task 10 registra os comandos sobre isto.

**Contexto:** a divergência que define este domínio é **validar na escrita, não na renderização**. O original valida ao renderizar (`FRACTAL_VIEW_RENDER_ERROR`), e o agente descobre que errou quando a tela aparece em branco. Aqui uma view inválida nunca é gravada e o erro nomeia o componente e a prop.

- [ ] **Step 1: Write the failing test — the five refusals**

`internal/domain/view/view_test.go`:

```go
// A view that would render blank is never written. Each refusal has to name
// what is wrong, because the reader is an agent that will try again
// immediately and needs to know what to change.
func TestAnUnknownComponentIsRefusedNamingIt(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "SuperTable"},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_COMPONENT_UNKNOWN" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["component"] != "SuperTable" {
		t.Fatalf("the error does not name the component: %v", app.Issues)
	}
}

func TestAMissingRequiredPropIsRefusedNamingIt(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	// Heading declares `text` as required in the catalog.
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "Heading", Props: map[string]any{"level": "h2"}},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_PROP_REQUIRED" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["prop"] != "text" {
		t.Fatalf("the error does not name the prop: %v", app.Issues)
	}
}
func TestAPropOfTheWrongTypeIsRefusedNamingIt(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "Heading", Props: map[string]any{"text": 7}},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_PROP_WRONG_TYPE" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["prop"] != "text" || app.Issues["expected"] != "string" {
		t.Fatalf("the error does not say what was expected: %v", app.Issues)
	}
}

// A bind points a prop at a field of the source collection. Pointing it at a
// field the collection does not declare renders nothing, silently.
func TestABindToAFieldTheCollectionDoesNotHaveIsRefused(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	// contacts declares name, email, stage. "salary" renders nothing, silently.
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "Text", Bind: map[string]string{"text": "salary"}},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_BIND_UNKNOWN_FIELD" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["field"] != "salary" {
		t.Fatalf("the error does not name the field: %v", app.Issues)
	}
}

// An action names a command from the registry. A view is not a parallel path
// to mutation: the same validation and the same authorisation apply.
func TestAnActionNamingAnUnregisteredCommandIsRefused(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()), withCommands("collections_records-delete"))
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{
			Component: "Button",
			Props:     map[string]any{"label": "Apagar tudo"},
			Actions:   []view.Action{{Label: "Apagar tudo", Command: "database_drop"}},
		},
	})
	// A view is not a parallel path to mutation: an action is a Descriptor
	// from the registry, with the same authorisation as any other surface.
	if code := codeOf(t, err); code != "AOS_VIEW_ACTION_COMMAND_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}

// A component that does not accept children may not have any.
func TestChildrenUnderALeafComponentAreRefused(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	// Badge declares no slots, so nothing can nest under it.
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{
			Component: "Badge",
			Children:  []view.Node{{Component: "Text", Props: map[string]any{"text": "x"}}},
		},
	})
	if code := codeOf(t, err); code != "AOS_VIEW_CHILDREN_NOT_ACCEPTED" {
		t.Fatalf("code = %q", code)
	}
}

// The refusal has to reach the deepest node, not only the root.
func TestAnInvalidNodeDeepInTheTreeIsStillRefused(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	_, err := svc.Create(ctx(), view.CreateInput{
		ID: "crm", Source: view.Source{Collection: "contacts"},
		Tree: view.Node{Component: "Stack", Children: []view.Node{
			{Component: "Card", Children: []view.Node{
				{Component: "Stack", Children: []view.Node{
					{Component: "NotAComponent"},
				}},
			}},
		}},
	})
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_VIEW_COMPONENT_UNKNOWN" {
		t.Fatalf("code = %q", app.Code)
	}
	// A view of thirty nodes whose error does not locate the bad one is an
	// error the agent cannot act on.
	at, _ := app.Issues["at"].(string)
	if !strings.Contains(at, "children") {
		t.Fatalf("the error does not say where in the tree: %v", app.Issues)
	}
}
```

E, do lado positivo:

```go
// Render resolves the source and returns the tree with the data attached —
// which is what the frontend's renderer consumes.
func TestRenderAttachesTheCollectionsRecords(t *testing.T) {
	svc := newService(t,
		withCollection(contactsSchema()),
		withRecords("contacts",
			map[string]any{"name": "Ada", "stage": "won"},
			map[string]any{"name": "Grace", "stage": "lead"},
		))
	mustCreateView(t, svc, "crm", view.Source{Collection: "contacts", Sort: []view.SortSpec{{Field: "name"}}})

	out, err := svc.Render(ctx(), view.RenderInput{ID: "crm"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Records) != 2 {
		t.Fatalf("rendered %d records, want 2", len(out.Records))
	}
	if out.Records[0].Data["name"] != "Ada" {
		t.Fatalf("the sort declared in Source was not applied: %v", out.Records)
	}
	if out.View.ID != "crm" {
		t.Fatal("the rendered payload does not carry the view itself")
	}
}

// Scaffold is what lets an agent produce a valid view without guessing: it
// infers components from the field types.
func TestScaffoldProducesAViewThatValidates(t *testing.T) {
	svc := newService(t, withCollection(contactsSchema()))
	v, err := svc.Scaffold(ctx(), view.ScaffoldInput{Collection: "contacts", Kind: view.KindTable})
	if err != nil {
		t.Fatal(err)
	}
	// The proof is not that it produced something; it is that what it produced
	// survives the same validation everything else goes through.
	if _, err := svc.Create(ctx(), view.CreateInput{ID: "scaffolded", Source: v.Source, Tree: v.Tree}); err != nil {
		t.Fatalf("Scaffold produced a view its own service refuses: %v", err)
	}
}

// Every component Scaffold can emit must exist in the generated catalog. This
// is the test that covers the difference between an embedded JSON catalog and
// a generated .gen.go: the compiler is not checking it, so this is.
func TestEveryComponentScaffoldCanEmitIsInTheCatalog(t *testing.T) {
	for _, name := range view.ScaffoldComponents() {
		if _, ok := view.LookupComponent(name); !ok {
			t.Fatalf("Scaffold emits %q, which is not in the catalog", name)
		}
	}
}

// ExecuteAction does not execute anything itself: it resolves the declared
// action and dispatches through the registry.
func TestExecuteActionDispatchesThroughTheRegistry(t *testing.T) {
	cmds := &fakeCommands{known: map[string]bool{"collections_records-delete": true}}
	svc := newService(t, withCollection(contactsSchema()), withCommandPort(cmds))
	mustCreateViewWithAction(t, svc, "crm", view.Action{
		Label: "Apagar", Command: "collections_records-delete",
		Input: map[string]any{"collection": "contacts"},
	})

	if _, err := svc.ExecuteAction(ctx(), view.ExecuteActionInput{
		ID: "crm", Label: "Apagar", Input: map[string]any{"id": "ada"},
	}); err != nil {
		t.Fatal(err)
	}
	// The view resolved and dispatched; it did not do the work itself.
	if len(cmds.invoked) != 1 || cmds.invoked[0] != "collections_records-delete" {
		t.Fatalf("invoked = %v", cmds.invoked)
	}
}

// An action the view does not declare cannot be executed, even by naming a
// real command — otherwise the view is a way to call anything.
func TestExecuteActionRefusesAnActionTheViewDoesNotDeclare(t *testing.T) {
	cmds := &fakeCommands{known: map[string]bool{"collections_records-delete": true}}
	svc := newService(t, withCollection(contactsSchema()), withCommandPort(cmds))
	mustCreateView(t, svc, "crm", view.Source{Collection: "contacts"})

	// Naming a real command is not enough. If it were, the view would be a way
	// to call anything in the registry from a button that was never declared.
	_, err := svc.ExecuteAction(ctx(), view.ExecuteActionInput{ID: "crm", Label: "Apagar"})
	if code := codeOf(t, err); code != "AOS_VIEW_ACTION_NOT_DECLARED" {
		t.Fatalf("code = %q", code)
	}
	if len(cmds.invoked) != 0 {
		t.Fatalf("it dispatched anyway: %v", cmds.invoked)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/domain/view/ -v`
Expected: FAIL nos novos, PASS nos do catálogo da Task 3.

- [ ] **Step 3: Write entity.go and port.go**

`Source`, `Node`, `Action`, `View`, `Kind` (`table`, `board`, `detail`), e os ports:

```go
// Collections is what the view domain needs to know about the data it binds
// to: the declaration, and the records. It is a port because internal/domain
// does not reach across to another domain's service directly.
type Collections interface {
	Get(ctx context.Context, id string) (*collection.Collection, error)
	ListRecords(ctx context.Context, id string, q collection.RecordQuery) ([]collection.Record, error)
}

// Commands is the slice of the command registry an Action needs: whether a
// command exists, and how to run one.
type Commands interface {
	Has(name string) bool
	Invoke(ctx context.Context, name string, in json.RawMessage) (json.RawMessage, error)
}
```

- [ ] **Step 4: Write validate.go**

`Validate(v View, c collection.Collection, cmds Commands) error`, que desce a árvore recursivamente. Para cada nó:

1. `LookupComponent(node.Component)` — falha → `errComponentUnknown` com `Issue("component", ...)`;
2. props: leia `spec.Props` (JSON Schema com `properties` e `required`) e cheque cada `required` presente, e o tipo de cada prop dada contra `type` do schema;
3. `bind`: cada valor tem que ser o nome de um `Field` de `c`;
4. `children`: se houver e `!spec.AcceptsChildren` → `errChildrenNotAccepted`;
5. `actions`: `cmds.Has(a.Command)` → falha → `errActionCommandUnknown`.

**Cuidado com a profundidade:** o erro tem que dizer *onde* na árvore. Carregue um caminho (`"tree.children[2].children[0]"`) e ponha-o em `Issue("at", path)` — sem isso, uma view de trinta nós dá um erro que não localiza nada.

- [ ] **Step 5: Write scaffold.go**

Mapa de tipo de campo para componente, e `ScaffoldComponents() []string` devolvendo todo componente que o scaffold pode emitir — é o que o teste do Step 1 verifica contra o catálogo:

```go
// scaffoldFor maps a declared field type to the component that shows it. It is
// deliberately conservative: a scaffold that renders is worth more than one
// that is clever, and the agent edits what it gets.
var scaffoldFor = map[collection.FieldType]string{
	collection.TypeString:  "Text",
	collection.TypeNumber:  "Stat",
	collection.TypeBoolean: "Badge",
	collection.TypeDate:    "Text",
	collection.TypeEnum:    "Badge",
	collection.TypeRef:     "Link",
	collection.TypeList:    "Text",
}
```

- [ ] **Step 6: Write service.go**

`Create` valida **antes** de persistir. `Render` busca a declaração da coleção, lista os registros com o filtro/ordem/limite do `Source`, e devolve `Rendered{View, Records, RenderedAt}`. `ExecuteAction` procura a `Action` pelo rótulo dentro da árvore da view, monta o input e chama `cmds.Invoke`.

- [ ] **Step 7: Run and commit**

```bash
go test -race ./internal/domain/view/ -v
task vet && task lint
git add internal/domain/view
git commit -m "feat(view): declarative screens, refused at write time when they would render blank"
```

---

### Task 8: O domínio `toolset` e o adaptador MCP stdio

**Files:**
- Create: `internal/domain/toolset/entity.go`
- Create: `internal/domain/toolset/errors.go`
- Create: `internal/domain/toolset/port.go`
- Create: `internal/domain/toolset/interpolate.go`
- Create: `internal/domain/toolset/service.go`
- Create: `internal/domain/toolset/toolset_test.go`
- Create: `internal/adapters/mcpclient/stdio.go`
- Create: `internal/adapters/mcpclient/stdio_test.go`
- Create: `internal/domain/testsuite/adapter.go`

**Interfaces:**
- Consumes: nada das tasks anteriores; o nativo `toolsets` veio na Task 4, Step 1.
- Produces:
  - `toolset.Toolset`, `toolset.Type` com as cinco constantes (só uma implementada), `toolset.Status`.
  - `toolset.Adapter` — o port de quatro métodos.
  - `toolset.ToolSpec` com `Name`, `Description`, `InputSchema map[string]any`.
  - `toolset.NewService(Deps) *Service` com `List`, `Get`, `Call`, `UpdateConfig`, `Delete`.
  - `testsuite.RunAdapterContract(t, AdapterContract)` — a suíte que os outros quatro tipos vão passar depois.
  - `mcpclient.NewStdio(...) toolset.Adapter`.

**Contexto:** o SDK oficial já é dependência — `internal/transport/mcpserver` usa `github.com/modelcontextprotocol/go-sdk/mcp` do lado **servidor**. Aqui é o lado **cliente**, que é novo. E a indireção é o ponto: as tools de um toolset **não** entram no registry do agente; ele as alcança por `toolsets_call`. A razão que mais importa das quatro é que a fronteira de execução externa fica auditável em um lugar só.

- [ ] **Step 1: Write the interpolation test first — it is the security-relevant part**

```go
// A missing secret fails the connection, naming the variable. Substituting an
// empty string instead produces a 401 much later, from a server, with nothing
// pointing back at the unset variable that caused it.
func TestInterpolationFailsLoudOnAMissingVariable(t *testing.T) {
	_, err := toolset.Interpolate("Bearer ${env.GITHUB_TOKEN}", envOf(map[string]string{}))
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_TOOLSET_ENV_MISSING" {
		t.Fatalf("code = %q", app.Code)
	}
	if app.Issues["variable"] != "GITHUB_TOKEN" {
		t.Fatalf("the error does not name the variable: %v", app.Issues)
	}
}

func TestInterpolationResolvesEveryOccurrence(t *testing.T) {
	env := envOf(map[string]string{"HOST": "api.example.com", "TOKEN": "abc"})
	got, err := toolset.Interpolate("https://${env.HOST}/v1?t=${env.TOKEN}&u=${env.HOST}", env)
	if err != nil {
		t.Fatal(err)
	}
	// Every occurrence, not just the first: a header that resolved once and
	// left the second placeholder literal is a request that fails opaquely.
	if got != "https://api.example.com/v1?t=abc&u=api.example.com" {
		t.Fatalf("got = %q", got)
	}
}
func TestATextWithNoPlaceholderIsReturnedUnchanged(t *testing.T) {
	env := envOf(map[string]string{})
	for _, in := range []string{"", "plain text", "$notenv", "${notenv.X}", "100% ${literal}"} {
		got, err := toolset.Interpolate(in, env)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got != in {
			t.Fatalf("%q became %q", in, got)
		}
	}
}

// A resolved secret must not end up in the error message of a later failure.
func TestAnInterpolatedValueIsNeverEchoedInAnError(t *testing.T) {
	env := envOf(map[string]string{"TOKEN": "super-secret-value"})
	ts := toolset.Toolset{
		ID: "gh", Type: toolset.MCPStdio, Command: "does-not-exist",
		Env: map[string]string{"AUTH": "${env.TOKEN}"},
	}
	svc := newService(t, withToolset(ts), withAdapter("gh", failingAdapter{}))

	_, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "x"})
	if err == nil {
		t.Fatal("a failing connection reported success")
	}
	// A resolved secret must not travel in the message of a later failure —
	// that message goes to a log, a UI, and possibly a bug report.
	if strings.Contains(err.Error(), "super-secret-value") {
		t.Fatalf("the resolved secret leaked into the error: %v", err)
	}
}
```

- [ ] **Step 2: Write the discriminated-union test**

```go
// The five types decode; an unknown type is refused rather than defaulting to
// one of them. Only stdio is implemented in this slice, and the other four
// have to fail with "not implemented here" rather than with a decode error —
// the difference matters to whoever reads it.
func TestTheFiveTypesDecodeAndAnUnknownOneIsRefused(t *testing.T) {
	for _, raw := range []string{
		"mcp-server::stdio", "mcp-server::http", "rest-api", "cli", "custom",
	} {
		if _, err := toolset.ParseType(raw); err != nil {
			t.Fatalf("%q did not decode: %v", raw, err)
		}
	}
	// An unknown type is refused rather than defaulting to one of them: a
	// toolset that silently became "custom" would connect to nothing and say
	// nothing about why.
	_, err := toolset.ParseType("grpc")
	if code := codeOf(t, err); code != "AOS_TOOLSET_TYPE_UNKNOWN" {
		t.Fatalf("code = %q", code)
	}
}
func TestATypeThisSliceDoesNotImplementSaysSo(t *testing.T) {
	svc := newService(t)
	_, err := svc.Call(ctx(), toolset.CallInput{ID: "rest-thing", Tool: "x"})
	if code := codeOf(t, err); code != "AOS_TOOLSET_TYPE_NOT_AVAILABLE" {
		t.Fatalf("code = %q", code)
	}
}
```

- [ ] **Step 3: Write the audit test — this is the reason the indirection exists**

```go
// Every external call is recorded: which toolset, which tool, how long, and
// whether it worked. Never the payload — it can carry the user's data, and an
// audit log that stores it becomes the thing that leaks.
func TestACallIsRecordedInActivityWithoutThePayload(t *testing.T) {
	acts := &fakeActivities{}
	svc := newService(t, withActivities(acts), withAdapter("gh", okAdapter{}))

	secret := `{"token":"super-secret-value"}`
	if _, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "create_issue",
		Input: json.RawMessage(secret)}); err != nil {
		t.Fatal(err)
	}

	if len(acts.published) != 1 {
		t.Fatalf("published %d activities, want 1", len(acts.published))
	}
	rendered := fmt.Sprintf("%+v", acts.published[0])
	if strings.Contains(rendered, "super-secret-value") {
		t.Fatalf("the payload reached the audit log:\n%s", rendered)
	}
	if !strings.Contains(rendered, "create_issue") {
		t.Fatalf("the audit record does not say which tool ran:\n%s", rendered)
	}
}

// A failed call is recorded too. An audit log that only holds successes is an
// audit log that hides exactly what somebody would look for.
func TestAFailedCallIsRecordedAsAFailure(t *testing.T) {
	acts := &fakeActivities{}
	svc := newService(t, withActivities(acts), withAdapter("gh", failingAdapter{}))

	if _, err := svc.Call(ctx(), toolset.CallInput{ID: "gh", Tool: "create_issue"}); err == nil {
		t.Fatal("a failing call reported success")
	}
	// An audit log that only holds successes hides exactly what somebody would
	// go looking for.
	if len(acts.published) != 1 {
		t.Fatalf("published %d activities, want 1", len(acts.published))
	}
	rendered := fmt.Sprintf("%+v", acts.published[0])
	if !strings.Contains(rendered, "failed") {
		t.Fatalf("the failure was not recorded as one:\n%s", rendered)
	}
}
```

- [ ] **Step 4: Run to verify they fail, then implement the domain**

Run: `go test ./internal/domain/toolset/ -v` → FAIL.

Escreva `entity.go`, `interpolate.go` (regex `\$\{env\.([A-Za-z_][A-Za-z0-9_]*)\}`), `port.go` com `Adapter` e `Adapters` (fábrica por tipo), `service.go`.

`Call` faz: `Get` → resolver o `Adapter` pelo tipo (não implementado → `errTypeNotAvailable`) → `Connect` → `Call` → registrar em Activity **com `defer`, para que a falha também seja registrada** → `Close`.

- [ ] **Step 5: Write the adapter contract suite**

`internal/domain/testsuite/adapter.go`, no molde de `RunRepositoryContract`:

```go
// AdapterContract is what every toolset connection type must satisfy. It
// exists now, with one implementer, so the other four types are added by
// passing this rather than by being reviewed by eye.
type AdapterContract struct {
	Name string
	New  func(t *testing.T) toolset.Adapter
	// Toolset is a configuration New's adapter can actually connect to.
	Toolset toolset.Toolset
	// ExpectTool is a tool the connected server is known to publish.
	ExpectTool string
}

func RunAdapterContract(t *testing.T, c AdapterContract)
```

A suíte prova: `Connect` num alvo válido; `ListTools` devolve pelo menos `ExpectTool`; `Call` de uma tool inexistente erra sem derrubar; `Close` é idempotente; `Connect` num alvo inválido erra em vez de bloquear para sempre.

- [ ] **Step 6: Write the MCP stdio adapter**

`internal/adapters/mcpclient/stdio.go`, usando o lado cliente de `github.com/modelcontextprotocol/go-sdk/mcp`. Confira a API do SDK contra `internal/transport/mcpserver/server.go`, que já usa o pacote — a versão em `go.mod` é a que vale.

**Cuidado:** este adaptador spawna processo. `internal/domain` não pode importar `os/exec`; por isso ele vive em `internal/adapters/`. E o binário do servidor MCP precisa estar declarado no manifesto da skill que o traz — a checagem entra na Task 9.

- [ ] **Step 7: Run the contract against a test MCP server**

`internal/adapters/mcpclient/stdio_test.go` sobe um servidor MCP de teste — o próprio `mcpserver` do projeto serve, o que também prova que os dois lados falam a mesma língua — e roda `testsuite.RunAdapterContract`.

- [ ] **Step 8: Run everything and commit**

```bash
go test -race ./internal/domain/toolset/ ./internal/adapters/mcpclient/ -v
task vet && task lint
git add internal/domain/toolset internal/adapters/mcpclient internal/domain/testsuite/adapter.go
git commit -m "feat(toolset): external tools behind one auditable boundary, over MCP stdio"
```

---

### Task 9: O domínio `skill` e o instalador

**Files:**
- Create: `internal/domain/skill/entity.go`
- Create: `internal/domain/skill/errors.go`
- Create: `internal/domain/skill/port.go`
- Create: `internal/domain/skill/verify.go`
- Create: `internal/domain/skill/install.go`
- Create: `internal/domain/skill/service.go`
- Create: `internal/domain/skill/skill_test.go`
- Create: `internal/adapters/skillfetch/local.go`
- Create: `internal/adapters/skillfetch/local_test.go`

**Interfaces:**
- Consumes: `collection.Service` (Task 4), `view.Service` (Task 7), `toolset.Service` (Task 8), `event.Broker` (já existe).
- Produces:
  - `skill.Skill` com `ID`, `Name`, `Description`, `Active`, `Version`, `Source`, `Commit`, `Permissions`, `Metadata`, `CreatedAt`, `UpdatedAt`, `Content`.
  - `skill.Permissions` e `skill.Metadata` conforme a nota `Skill (Go)`.
  - `skill.Fetcher` — `Fetch(ctx, source, ref string) (Package, error)`.
  - `skill.Package` — o conteúdo lido, ainda não aplicado.
  - `skill.Verifier` — `VerifyManifest(Package) (Diff, error)`.
  - `skill.InstallInput` com `Source string`, `Ref string`, `AcceptedAll func(Permissions) bool` — nil significa "pergunte".
  - `skill.NewInstaller(Deps) *Installer` com `Install`, `Uninstall`, `Get`, `Views()`.
  - `skillfetch.Local` implementando `skill.Fetcher` a partir de um diretório.

**Contexto crítico — o lugar já existe.** `internal/core/collections/registry.go` já declara o nativo `skills` com `CascadeDelete: true`, e já declara os segundos padrões que fazem uma skill trazer agentes, memórias, rotinas, templates, instruções e goals. **Não redeclare nada disso.** O que falta é o domínio.

- [ ] **Step 1: Write the manifest-refusal test — it is why ADR-0015 exists**

```go
// The manifest is not documentation: content that exceeds it is refused, and
// the refusal names the excess. A package that declared no exec permission and
// ships a toolset that runs a binary is exactly the case this closes.
func TestContentExceedingTheManifestIsRefusedNamingTheExcess(t *testing.T) {
	pkg := packageWith(
		skill.Permissions{Collections: []string{"contacts"}},
		withCollection("contacts"),
		withCollection("deals"), // not declared
	)
	_, err := skill.NewVerifier().VerifyManifest(pkg)
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_SKILL_MANIFEST_EXCEEDED" {
		t.Fatalf("code = %q", app.Code)
	}
	if !strings.Contains(fmt.Sprint(app.Issues["excess"]), "deals") {
		t.Fatalf("the refusal does not name the excess: %v", app.Issues)
	}
}

func TestAToolsetRunningAnUndeclaredBinaryIsRefused(t *testing.T) {
	// The package ships an mcp-server::stdio toolset that spawns `curl`, and
	// declares no exec permission for it. Two doors, both closed by default:
	// the binary has to be in the agent's sandbox allowlist *and* declared in
	// the skill's manifest.
	pkg := packageWith(
		skill.Permissions{Exec: []string{"gh"}},
		withToolsetCommand("curl"),
	)
	_, err := skill.NewVerifier().VerifyManifest(pkg)
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if app.Code != "AOS_SKILL_MANIFEST_EXCEEDED" {
		t.Fatalf("code = %q", app.Code)
	}
	if !strings.Contains(fmt.Sprint(app.Issues["excess"]), "curl") {
		t.Fatalf("the refusal does not name the binary: %v", app.Issues)
	}
}
func TestAHostOutsideThePermissionsNetworkListIsRefused(t *testing.T) {
	pkg := packageWith(
		skill.Permissions{Network: []string{"api.github.com"}},
		withToolsetURL("https://evil.example.com/mcp"),
	)
	_, err := skill.NewVerifier().VerifyManifest(pkg)
	var app *apperr.Error
	if !errors.As(err, &app) {
		t.Fatalf("err is %T", err)
	}
	if !strings.Contains(fmt.Sprint(app.Issues["excess"]), "evil.example.com") {
		t.Fatalf("the refusal does not name the host: %v", app.Issues)
	}
}
```

- [ ] **Step 2: Write the consent test**

```go
// An agent calling skills_install does not authorise itself. The request goes
// through the approval channel at high risk, and a denial means nothing was
// written.
func TestInstallingWithoutConsentIsRefusedAndWritesNothing(t *testing.T) {
	approver := denyingApprover{}
	inst := newInstaller(t, withApprover(approver))

	_, err := inst.Install(ctx(), skill.InstallInput{Source: fixtureDir(t, "crm-skill")})
	if code := codeOf(t, err); code != "AOS_SKILL_INSTALL_NOT_APPROVED" {
		t.Fatalf("code = %q", code)
	}
	if approver.lastRisk != event.RiskHigh {
		t.Fatalf("risk = %q, want high", approver.lastRisk)
	}
	if wrote := filesWritten(t); len(wrote) != 0 {
		t.Fatalf("a refused install left files behind: %v", wrote)
	}
}

// The order matters: nothing touches the workspace before the manifest is
// verified and a human has consented.
func TestNothingIsWrittenBeforeVerificationAndConsent(t *testing.T) {
	var order []string
	inst := newInstaller(t,
		withVerifier(recordingVerifier{&order}),
		withApprover(recordingApprover{&order, true}),
		withApplier(recordingApplier{&order}),
	)

	if _, err := inst.Install(ctx(), skill.InstallInput{Source: fixtureDir(t, "crm-skill")}); err != nil {
		t.Fatal(err)
	}
	want := []string{"verified", "asked", "applied"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

// Registration is last, so a partial failure leaves an unregistered directory
// rather than a half-registered skill.
func TestAFailureWhileApplyingLeavesNothingRegistered(t *testing.T) {
	reg := collections.NewRegistry()
	inst := newInstaller(t,
		withRegistry(reg),
		withApprover(approvingApprover{}),
		withApplier(applierFailingAt("views")),
	)

	if _, err := inst.Install(ctx(), skill.InstallInput{Source: fixtureDir(t, "crm-skill")}); err == nil {
		t.Fatal("a failing apply reported success")
	}
	// Registration is last, so a partial failure leaves an unregistered
	// directory rather than a half-registered skill — the first is something a
	// person can delete, the second is something they cannot reason about.
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the collection stayed registered after the install failed")
	}
	if _, err := inst.Get(ctx(), "crm"); err == nil {
		t.Fatal("the skill is listed as installed after a failed install")
	}
}
```

- [ ] **Step 3: Write the uninstall test**

```go
// Uninstalling removes what came with the skill — the whole directory, which
// the native's CascadeDelete already does. What this test is really about is
// the ordering: hooks and toolsets are deregistered *before* the files go, so
// nothing is left registered pointing at a directory that no longer exists.
func TestUninstallDeregistersBeforeRemovingFiles(t *testing.T) {
	var order []string
	inst := newInstaller(t,
		withApprover(approvingApprover{}),
		withHooks(recordingHooks{&order}),
		withRemover(recordingRemover{&order}),
	)
	installed := mustInstall(t, inst, "crm-skill")

	if err := inst.Uninstall(ctx(), skill.UninstallInput{ID: installed.ID}); err != nil {
		t.Fatal(err)
	}
	want := []string{"hooks deregistered", "toolsets closed", "files removed"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v — nothing may stay registered pointing at a directory that is gone", order, want)
	}
}

// A skill-scoped collection and view go with it.
func TestUninstallRemovesTheCollectionsAndViewsTheSkillBrought(t *testing.T) {
	reg := collections.NewRegistry()
	inst := newInstaller(t, withRegistry(reg), withApprover(approvingApprover{}))
	installed := mustInstall(t, inst, "crm-skill")

	if _, ok := reg.Lookup("contacts"); !ok {
		t.Fatal("the skill did not bring its collection")
	}
	if err := inst.Uninstall(ctx(), skill.UninstallInput{ID: installed.ID}); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Lookup("contacts"); ok {
		t.Fatal("the skill-scoped collection outlived the skill")
	}
	if _, err := inst.Views().Get(ctx(), "contacts-table"); err == nil {
		t.Fatal("the skill-scoped view outlived the skill")
	}
}
```

- [ ] **Step 4: Run to verify they fail, then write the domain**

Run: `go test ./internal/domain/skill/ -v` → FAIL.

`install.go` implementa exatamente a ordem da spec:

```go
// Install fetches, verifies and applies a skill package.
//
// The order is the decision, not an implementation detail. Nothing touches the
// workspace before the manifest is verified and a human has consented, and
// registration happens last so a partial failure leaves an unregistered
// directory rather than a half-registered skill.
func (i *Installer) Install(ctx context.Context, in InstallInput) (*Skill, error) {
	pkg, err := i.fetcher.Fetch(ctx, in.Source, in.Ref)
	if err != nil {
		return nil, err
	}
	diff, err := i.verifier.VerifyManifest(pkg)
	if err != nil {
		return nil, err
	}
	// AcceptedAll is a field, not a method: it is how a caller that already has
	// consent — a CLI run with --yes, or a test — says so, and a func field
	// keeps that decision at the call site instead of inside the input type.
	if in.AcceptedAll == nil || !in.AcceptedAll(diff.Permissions) {
		res, err := i.approver.RequestApproval(ctx, event.ApprovalRequest{
			ToolName: "skills_install",
			Risk:     event.RiskHigh,
			Reason:   diff.Render(),
		})
		if err != nil {
			return nil, err
		}
		if !res.Approved {
			return nil, errInstallNotApproved(in.Source, res.Reason)
		}
	}
	return i.apply(ctx, pkg)
}
```

- [ ] **Step 5: Write the local fetcher**

`internal/adapters/skillfetch/local.go` lê um diretório: `SKILL.md` mais o que houver em `agents/`, `collections/`, `views/`, `toolsets/`, `templates/`, `instructions/`, `goals/`, `references/`.

`Version`, `Source` e `Commit` ficam **vazios** e isso está declarado no escopo: com fonte local não há proveniência a registrar, e preencher com algo inventado seria pior que vazio.

**Cuidado:** o fetcher confina a leitura ao diretório dado, com `pathx.ResolveInside` — um `SKILL.md` com um caminho que sobe é um pacote lendo fora de si.

- [ ] **Step 6: Build the fixture**

`internal/domain/skill/testdata/crm-skill/` com um `SKILL.md` cujo manifesto declara exatamente o que o pacote traz: um agente, a coleção `contacts` e uma view. É a mesma fixture que a Task 11 usa — construa-a aqui, com cuidado, e não a duplique lá.

- [ ] **Step 7: Run and commit**

```bash
go test -race ./internal/domain/skill/ ./internal/adapters/skillfetch/ -v
task vet && task lint
git add internal/domain/skill internal/adapters/skillfetch
git commit -m "feat(skill): install a capability, with the manifest verified and consent asked"
```

---

### Task 10: Comandos, fiação e o acendimento dos dormentes

**Files:**
- Create: `internal/domain/collection/commands.go`
- Create: `internal/domain/view/commands.go`
- Create: `internal/domain/toolset/commands.go`
- Create: `internal/domain/skill/commands.go`
- Modify: `internal/app/wire.go`
- Modify: `frontend/src/lib/command-map.ts`
- Modify: `frontend/src/lib/schema.ts` (gerado — via `task gen-schema`)

**Interfaces:**
- Consumes: os quatro serviços das Tasks 4, 5, 7, 8, 9.
- Produces: 28 comandos no registry, e quatro domínios acesos no frontend.

**Contexto:** o contrato de aceitação desta fatia é o `COMMAND_MAP`. Enquanto houver `null` nestes quatro domínios, ela não fechou. Escreva os comandos **com os nomes que a UI já chama** — a tabela abaixo é a lista fechada, extraída do `command-map.ts` de hoje.

| Grupo | Comandos que a UI chama | Comandos só de agente/CLI |
|---|---|---|
| `collections` | `list`, `get`, `delete`, `records-list`, `records-get`, `records-create`, `records-update`, `records-delete` | `create` |
| `views` | `list`, `get`, `render`, `execute-action`, `delete` | `create`, `components`, `scaffold` |
| `skills` | `list`, `install`, `update`, `delete` | `create` |
| `toolsets` | `get`, `get-config`, `update-config`, `delete` | `list`, `call` |

- [ ] **Step 1: Write commands.go for each domain**

Siga `internal/domain/theme/commands.go` como molde: `GroupDoc` com `Name`, `Tool`, `Summary` e um `Doc` em Markdown listando os comandos; depois um `command.MustRegister` por comando, com `Summary`, `Doc`, pelo menos dois `Examples`, `Registry: true` e `Annotations` honestas.

**As anotações não são decoração — elas derivam o risco de aprovação do ADR-0007.** `ReadOnlyHint: true` num comando que escreve é um comando que passa sem perguntar.

```go
	command.MustRegister(reg, command.Command[InstallInput, InstallOutput]{
		Group:   "skills",
		Name:    "install",
		Summary: "Install a skill package.",
		Doc: `Installs a capability: the agents, collections, views, routines and
memories it ships, as one unit.

The package's manifest is verified against what it actually contains before
anything is written, and a person is asked before it is applied. An agent does
not authorise this on its own.`,
		Examples: []command.Example{
			{Description: "install from a local directory", Input: InstallInput{Source: "./skills/crm"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Install a skill", DestructiveHint: true, OpenWorldHint: true},
		Handler:     svc.Install,
	})
```

- [ ] **Step 2: Regenerate the TypeScript and confirm the count moved**

```bash
task gen-schema && git diff --stat frontend/src/lib/schema.ts
```
Expected: o cabeçalho do gerado passa de 71 comandos para 99.

- [ ] **Step 3: Wire the four services**

Em `internal/app/wire.go`: construa o `*collections.Registry` uma vez, passe-o a `collection.NewService`; monte os repositórios dos quatro nativos novos com `fscollections.New` mais `WithLock`, `WithIndex` e `WithPublisher`; e acrescente os quatro `Register`.

Exporte o registry no `App` — a Task 6 precisa dele e o teste da Task 11 também:

```go
	// CollectionRegistry is what dynamic collections are registered into. It is
	// exported for the same reason Clock is: Serve and the watcher build on it,
	// and a test needs to assert that a collection an agent created is actually
	// reachable.
	//
	// It is deliberately not called Collections — that name belongs to the
	// service, and two fields one letter apart is how a test ends up asserting
	// against the wrong one.
	CollectionRegistry *collections.Registry

	// Collections is the collection domain: declarations and their records.
	Collections *collection.Service

	// Views, Toolsets and Skills complete the ecosystem slice.
	Views    *view.Service
	Toolsets *toolset.Service
	Skills   *skill.Installer
```

- [ ] **Step 4: Light up the frontend**

Em `frontend/src/lib/command-map.ts`:

```ts
export const DORMANT_DOMAINS: ReadonlySet<string> = new Set([
  "artifact", "goal", "instruction", "marketplace", "model",
  "project", "template", "token", "user",
]);
```

E substitua os 21 `null` pelos caminhos reais, por exemplo:

```ts
  "collection.listRecords": "collections.records-list",
  "view.executeAction": "views.execute-action",
```

- [ ] **Step 5: Prove there are no nulls left in these four domains**

```bash
cd frontend && node -e '
const src = require("fs").readFileSync("src/lib/command-map.ts", "utf8");
const left = [...src.matchAll(/"(collection|view|skill|toolset)\.[a-zA-Z]+":\s*null/g)];
if (left.length) { console.error("ainda dormentes:", left.map(m => m[0])); process.exit(1); }
console.log("os quatro domínios estão acesos");
'
```
Expected: `os quatro domínios estão acesos`

- [ ] **Step 6: Exercise it over real HTTP, not by reading**

Suba o daemon e chame os 21 caminhos com `curl`, conferindo que nenhum devolve 404 de comando desconhecido. A lição registrada no ledger da fase anterior é exatamente esta: a varredura mecânica passou e o caminho real estava quebrado.

- [ ] **Step 7: Run every gate and commit**

```bash
task check
cd frontend && npx tsc --noEmit && npx vitest run
git add internal/domain/*/commands.go internal/app/wire.go frontend/src/lib/command-map.ts frontend/src/lib/schema.ts
git commit -m "feat: register the four ecosystem domains and light them up in the interface"
```

---

### Task 11: `TestTheDeliveryOfPhaseEight`

**Files:**
- Create: `internal/app/ecosystem_test.go`
- Modify: `docs/04 - Domínio/Collection (Go).md`, `View (Go).md`, `Toolset (Go).md`, `Skill (Go).md` (estado da fase)
- Modify: `docs/08 - Entrega/Roteiro de Fases.md`

**Interfaces:**
- Consumes: tudo.
- Produces: a afirmação sobre a qual a fase é julgada.

- [ ] **Step 1: Write the delivery test**

`internal/app/ecosystem_test.go`, no molde de `journey_test.go` — disco real, não fakes:

```go
// TestTheDeliveryOfPhaseEight is the claim this slice is judged on: a skill
// installs a capability, the collection it brought is usable in the same
// session that created it, the view it brought renders with real data, and
// uninstalling takes all three away.
//
// Every assertion is about a file that exists or a record that comes back,
// because the phase promises a system that works and not a set of packages
// that compile.
func TestTheDeliveryOfPhaseEight(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	a := newAppAt(t, home, repo, "")
	t.Cleanup(func() { _ = a.Close() })

	// Step one: install the skill from a local directory, with consent given.
	installed, err := a.Skills.Install(ctx(), skill.InstallInput{
		Source:      filepath.Join("testdata", "crm-skill"),
		AcceptedAll: acceptEverything,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("the skill brought an agent of its own", func(t *testing.T) {
		agents, err := a.Agents.List(ctx(), agent.ListInput{})
		if err != nil {
			t.Fatal(err)
		}
		var found *agent.Agent
		for i := range agents.Agents {
			if agents.Agents[i].Skill == installed.ID {
				found = &agents.Agents[i]
			}
		}
		if found == nil {
			t.Fatal("no agent came with the skill")
		}
	})

	t.Run("the collection is usable in the same session, with no restart", func(t *testing.T) {
		if _, ok := a.CollectionRegistry.Lookup("contacts"); !ok {
			t.Fatal("the collection the skill brought is not registered")
		}
		rec, err := a.Collections.Records().Create(ctx(), "contacts", map[string]any{
			"name": "Ada Lovelace", "stage": "lead",
		})
		if err != nil {
			t.Fatal(err)
		}
		on := filepath.Join(repo, collections.Root, "collections", "contacts", "records", rec.ID+".md")
		if _, err := os.Stat(on); err != nil {
			t.Fatalf("the record is not on disk at %s: %v", on, err)
		}
	})

	t.Run("the view renders with the record attached", func(t *testing.T) {
		out, err := a.Views.Render(ctx(), view.RenderInput{ID: "contacts-table"})
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Records) != 1 {
			t.Fatalf("the view rendered %d records, want 1", len(out.Records))
		}
		if out.Records[0].Data["name"] != "Ada Lovelace" {
			t.Fatalf("the view rendered %v", out.Records[0].Data)
		}
	})

	t.Run("uninstalling takes all three away", func(t *testing.T) {
		if err := a.Skills.Uninstall(ctx(), skill.UninstallInput{ID: installed.ID}); err != nil {
			t.Fatal(err)
		}
		if _, ok := a.CollectionRegistry.Lookup("contacts"); ok {
			t.Fatal("the collection is still registered after uninstalling")
		}
		dir := filepath.Join(repo, collections.Root, "skills", installed.ID)
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("the skill directory survived: %v", err)
		}
	})
}
```

**Os nomes de campo são os da Task 10, Step 3** — `App.CollectionRegistry` é o `*collections.Registry` e `App.Collections` é o serviço. Não são intercambiáveis, e dois campos a uma letra de distância seriam um teste afirmando contra o objeto errado.

- [ ] **Step 2: Run it**

Run: `go test -race ./internal/app/ -run TestTheDeliveryOfPhaseEight -v`
Expected: PASS, com os quatro subtestes.

- [ ] **Step 3: Update the domain notes**

Cada uma das quatro notas em `docs/04 - Domínio/` ganha uma seção `## Estado — Fase 8` dizendo o que ficou pronto e **o que não ficou, nomeado**. As quatro permanecem `em-construcao`: `toolset` tem um tipo de cinco, `skill` não busca de fonte remota, e `collection`/`view` dependem das duas.

Não marque `pronto` o que não está. A Fase 7 mostrou o custo: três caixas marcadas escondendo a lacuna mais séria da fase.

- [ ] **Step 4: Update the roadmap**

Em `docs/08 - Entrega/Roteiro de Fases.md`, a entrada da Fase 8 vira **Parcialmente entregue**, com a fatia do núcleo descrita e os oito domínios restantes nomeados como o que falta.

- [ ] **Step 5: Every gate, and then say so with the output attached**

```bash
task check
cd frontend && npx tsc --noEmit && npx vitest run && cd ..
task build:desktop && task build:all
```

A "Definição de pronto por fase" do roteiro pede a saída anexada à nota. Anexe-a de verdade — não escreva "verde".

- [ ] **Step 6: Commit and tag**

```bash
git add -A
git commit -m "test(app): TestTheDeliveryOfPhaseEight — a skill installs a capability"
git tag -a v0.10.0-fase8-nucleo -F <(printf '%s\n' 'v0.10.0-fase8-nucleo — o núcleo do Ecossistema')
```

---

## Notas para quem executar

**A ordem importa e não é arbitrária.** Tasks 1 e 2 mexem no motor que 14 domínios usam; se elas não estiverem verdes com `task test` inteiro, nada depois vale. Task 3 é independente das outras duas e pode ser feita em paralelo. Tasks 4 a 9 dependem de 1, 2 e 3. Task 10 depende de todas.

**Três armadilhas concretas, tiradas do que já deu errado neste repositório:**

1. **`git diff --exit-code` num gerado só protege se a geração for determinística.** Ordene tudo que emitir. O `catalog.gen.go` já vazou uma vez por line numbers que mudaram sem regeração.
2. **Um gate que não exercita o caminho real não é um gate.** A fase anterior tinha cinco gates verdes e `wails3 dev` quebrado, porque nenhum deles percorria o caminho de instalação do dev. Se você acrescentar um gate, pergunte que caminho ele percorre.
3. **Não classifique um teste vermelho como "pré-existente e não relacionado" sem `git log -S`.** Cinco testes atravessaram quatro tasks assim na fase anterior.

**Sobre escrever comentário neste repositório:** os comentários aqui explicam *por que*, não *o quê*, e frequentemente registram a alternativa rejeitada. Siga isso — é o que faz as decisões sobreviverem a quem as tomou.
