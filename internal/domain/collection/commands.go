package collection

import (
	"context"
	"strings"

	"github.com/OWNER/aos/internal/core/command"
)

// GroupDoc is what a model reads before choosing this group.
var GroupDoc = command.GroupDoc{
	Name:    "collections",
	Tool:    "Collections",
	Summary: "Structured tables an agent declares at runtime.",
	Doc: `Data an agent shapes without a programmer: a schema, and the rows that
follow it.

A collection is a declaration — its fields, their types, what a hook
normalises on write — and its records are the rows that declaration governs.
"build me a CRM" becomes a collections_create for "contacts" plus the records
an agent or a person files into it, nothing generated, nothing deployed.

## Commands
- **list** — every collection declared in the workspace, native and dynamic
- **get** — one declaration, including a skill-scoped one (pass skill)
- **create** — declare a new collection
- **delete** — remove a declaration and unregister it
- **records-list** — the rows of one collection
- **records-get** — one row
- **records-create** — file a new row
- **records-update** — revalidate and rewrite a row
- **records-delete** — remove a row

## When to use
- **Structured data with no fixed shape in this codebase:** declare a
  collection before inventing a file format for it
- **A skill's own collection:** list returns it with its Skill field; get and
  delete need that field back to resolve it

## When NOT to use
- Not for anything already native (tasks, agents, memories) — those have
  their own groups and their own validation
- Not for one-off, unstructured content — that is a file`,
	Hint: `A record is validated against its collection's schema on every write, not
only when it looks wrong: a hook runs before validation, so what fails or
passes is what was actually stored, never what was typed before the hook
touched it.

A skill-scoped collection is removed automatically when its skill is
uninstalled — deleting it directly, by hand, is rarely what you want while
the skill is still installed.`,
}

// DeleteOutput confirms what was removed. Service.Delete itself returns only
// an error — a collection has no field CascadeDelete could partially fail to
// remove and worth reporting back — so this exists purely as the shape the
// command surface answers with, echoing what was asked for.
type DeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the collection that was removed."`
}

// RecordsListInput selects the rows of one collection. Skill is not part of
// this shape: RecordService resolves a collection's records by id alone
// (Records() shares the declaration's own repository, already bound to its
// format and pattern), unlike Service.Get/Delete above, which need Skill to
// find a skill-scoped declaration in the first place.
type RecordsListInput struct {
	Collection string         `json:"collection" jsonschema:"Id of the collection to list records from." validate:"required,notblank"`
	Filters    map[string]any `json:"filters,omitempty" jsonschema:"Field values a record must match, by field name."`
	OrderBy    string         `json:"orderBy,omitempty" jsonschema:"Name of a declared field to order by. Empty orders by path, which is stable."`
	Desc       bool           `json:"desc,omitempty" jsonschema:"Reverse the order."`
	Limit      int            `json:"limit,omitempty" jsonschema:"Maximum number of records to return. 0 means no limit."`
	Offset     int            `json:"offset,omitempty" jsonschema:"Number of matching records to skip, for pagination."`

	command.Reasoning
}

// RecordsListOutput is the rows RecordsListInput matched.
type RecordsListOutput struct {
	Records []Record `json:"records" jsonschema:"The matching rows."`
	Total   int      `json:"total" jsonschema:"How many were returned — not how many exist, when Limit cut the list short."`
}

// RecordsGetInput names one row of one collection.
type RecordsGetInput struct {
	Collection string `json:"collection" jsonschema:"Id of the record's collection." validate:"required,notblank"`
	ID         string `json:"id" jsonschema:"Identifier of the record." validate:"required,notblank"`

	command.Reasoning
}

// RecordsCreateInput files a new row.
type RecordsCreateInput struct {
	Collection string         `json:"collection" jsonschema:"Id of the collection to file the record into." validate:"required,notblank"`
	Data       map[string]any `json:"data" jsonschema:"The record's fields, as declared by its collection's schema."`
	Content    string         `json:"content,omitempty" jsonschema:"The Markdown body, for a collection of format md. Ignored for json."`

	command.Reasoning
}

// RecordsUpdateInput revalidates and rewrites a row. Content is not one of
// its fields for the reason RecordService.Update's own doc gives: it is not
// a field, and Update has no way to change it — the stored body carries
// over untouched.
type RecordsUpdateInput struct {
	Collection string         `json:"collection" jsonschema:"Id of the record's collection." validate:"required,notblank"`
	ID         string         `json:"id" jsonschema:"Identifier of the record to rewrite." validate:"required,notblank"`
	Data       map[string]any `json:"data" jsonschema:"The record's new fields, replacing the old ones wholesale."`

	command.Reasoning
}

// RecordsDeleteInput names the row to remove.
type RecordsDeleteInput struct {
	Collection string `json:"collection" jsonschema:"Id of the record's collection." validate:"required,notblank"`
	ID         string `json:"id" jsonschema:"Identifier of the record to remove." validate:"required,notblank"`

	command.Reasoning
}

// RecordsDeleteOutput confirms what was removed.
type RecordsDeleteOutput struct {
	ID string `json:"id" jsonschema:"Identifier of the record that was removed."`
}

// Register declares the group on the registry.
func Register(reg *command.Registry, svc *Service) {
	reg.DescribeGroup(GroupDoc)

	command.MustRegister(reg, command.Command[ListInput, ListOutput]{
		Group:   "collections",
		Name:    "list",
		Summary: "List every declared collection.",
		Doc:     "Every collection declared in the workspace — native and dynamic, workspace-scoped and skill-scoped alike.",
		Examples: []command.Example{
			{Description: "everything declared", Input: ListInput{}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List collections", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.List,
	})

	command.MustRegister(reg, command.Command[GetInput, *Collection]{
		Group:   "collections",
		Name:    "get",
		Summary: "Read one collection's declaration.",
		Doc: `Read one collection: its fields, their types, and the hooks that
normalise a record on write.

A skill-scoped collection needs its Skill back to resolve — collections_list
reports it on every entry, so a caller that listed first already has it.`,
		Examples: []command.Example{
			{Description: "a workspace collection", Input: GetInput{ID: "contacts"}},
			{Description: "a collection a skill brought", Input: GetInput{ID: "contacts", Skill: "crm"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a collection", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     svc.Get,
	})

	command.MustRegister(reg, command.Command[CreateInput, *Collection]{
		Group:   "collections",
		Name:    "create",
		Summary: "Declare a new collection.",
		Doc: `Declare a table: its fields and, optionally, the hooks that normalise a
record before it is validated.

The schema is checked for internal coherence — every ref points at something
real, every enum has values, no field is named twice — before anything is
written, and the collection is registered only once the write to disk has
actually succeeded.`,
		Examples: []command.Example{
			{
				Description: "a contacts table",
				Input: CreateInput{
					ID: "contacts", Name: "Contacts", Format: FormatJSON,
					Fields: []Field{
						{Name: "name", Type: TypeString, Required: true},
						{Name: "email", Type: TypeString, Unique: true},
					},
				},
			},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Declare a collection"},
		Handler:     svc.Create,
	})

	deleteHandler := func(ctx context.Context, in DeleteInput) (DeleteOutput, error) {
		id := strings.TrimSpace(in.ID)
		if err := svc.Delete(ctx, in); err != nil {
			return DeleteOutput{}, err
		}
		return DeleteOutput{ID: id}, nil
	}
	command.MustRegister(reg, command.Command[DeleteInput, DeleteOutput]{
		Group:   "collections",
		Name:    "delete",
		Summary: "Remove a collection's declaration.",
		Doc: `Remove a declaration and unregister it. Its records go with it — nothing
can resolve a write against a declaration that no longer exists.

A skill-scoped collection is normally removed by uninstalling the skill, not
by calling this directly while the skill is still installed.`,
		Examples: []command.Example{
			{Description: "remove a workspace collection", Input: DeleteInput{ID: "contacts"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a collection", DestructiveHint: true},
		Handler:     deleteHandler,
	})

	recordsListHandler := func(ctx context.Context, in RecordsListInput) (RecordsListOutput, error) {
		rows, err := svc.Records().List(ctx, in.Collection, RecordQuery{
			Filters: in.Filters, OrderBy: in.OrderBy, Desc: in.Desc, Limit: in.Limit, Offset: in.Offset,
		})
		if err != nil {
			return RecordsListOutput{}, err
		}
		return RecordsListOutput{Records: rows, Total: len(rows)}, nil
	}
	command.MustRegister(reg, command.Command[RecordsListInput, RecordsListOutput]{
		Group:   "collections",
		Name:    "records-list",
		Summary: "List a collection's rows.",
		Doc:     "Every row of one collection matching the given filters and order.",
		Examples: []command.Example{
			{Description: "every row", Input: RecordsListInput{Collection: "contacts"}},
			{Description: "filtered and ordered", Input: RecordsListInput{
				Collection: "contacts", Filters: map[string]any{"stage": "qualified"}, OrderBy: "name",
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "List records", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     recordsListHandler,
	})

	recordsGetHandler := func(ctx context.Context, in RecordsGetInput) (*Record, error) {
		return svc.Records().Get(ctx, in.Collection, in.ID)
	}
	command.MustRegister(reg, command.Command[RecordsGetInput, *Record]{
		Group:   "collections",
		Name:    "records-get",
		Summary: "Read one row.",
		Doc:     "Read one record of a collection in full.",
		Examples: []command.Example{
			{Description: "a contact by id", Input: RecordsGetInput{Collection: "contacts", ID: "c-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Read a record", ReadOnlyHint: true, IdempotentHint: true},
		Handler:     recordsGetHandler,
	})

	recordsCreateHandler := func(ctx context.Context, in RecordsCreateInput) (*Record, error) {
		return svc.Records().CreateWithContent(ctx, in.Collection, in.Data, in.Content)
	}
	command.MustRegister(reg, command.Command[RecordsCreateInput, *Record]{
		Group:   "collections",
		Name:    "records-create",
		Summary: "File a new row.",
		Doc: `Add a record to a collection.

The declared hooks normalise the data first, then it is validated against the
schema and, for a field declared unique, against what is already stored.`,
		Examples: []command.Example{
			{Description: "a new contact", Input: RecordsCreateInput{
				Collection: "contacts", Data: map[string]any{"name": "Ada Lovelace", "email": "ada@example.com"},
			}},
			{Description: "a note with a Markdown body", Input: RecordsCreateInput{
				Collection: "notes", Data: map[string]any{"title": "Kickoff"}, Content: "## Agenda\n\n- scope\n- owners",
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Create a record"},
		Handler:     recordsCreateHandler,
	})

	recordsUpdateHandler := func(ctx context.Context, in RecordsUpdateInput) (*Record, error) {
		return svc.Records().Update(ctx, in.Collection, in.ID, in.Data)
	}
	command.MustRegister(reg, command.Command[RecordsUpdateInput, *Record]{
		Group:   "collections",
		Name:    "records-update",
		Summary: "Revalidate and rewrite a row.",
		Doc: `Replace a record's fields and revalidate them. A refusal leaves the
stored record untouched — nothing between reading it and validating the new
data can have written anything.`,
		Examples: []command.Example{
			{Description: "move a contact to a new stage", Input: RecordsUpdateInput{
				Collection: "contacts", ID: "c-1", Data: map[string]any{"name": "Ada Lovelace", "stage": "qualified"},
			}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Update a record"},
		Handler:     recordsUpdateHandler,
	})

	recordsDeleteHandler := func(ctx context.Context, in RecordsDeleteInput) (RecordsDeleteOutput, error) {
		if err := svc.Records().Delete(ctx, in.Collection, in.ID); err != nil {
			return RecordsDeleteOutput{}, err
		}
		return RecordsDeleteOutput{ID: in.ID}, nil
	}
	command.MustRegister(reg, command.Command[RecordsDeleteInput, RecordsDeleteOutput]{
		Group:   "collections",
		Name:    "records-delete",
		Summary: "Remove a row.",
		Doc:     "Remove one record from a collection.",
		Examples: []command.Example{
			{Description: "remove a contact", Input: RecordsDeleteInput{Collection: "contacts", ID: "c-1"}},
		},
		Registry:    true,
		Annotations: command.Annotations{Title: "Delete a record", DestructiveHint: true},
		Handler:     recordsDeleteHandler,
	})
}

// compile-time proof that the handlers match the command signature.
var (
	_ func(context.Context, ListInput) (ListOutput, error)   = (*Service)(nil).List
	_ func(context.Context, GetInput) (*Collection, error)   = (*Service)(nil).Get
	_ func(context.Context, CreateInput) (*Collection, error) = (*Service)(nil).Create
	_ func(context.Context, DeleteInput) error                = (*Service)(nil).Delete
)
