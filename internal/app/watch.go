package app

import (
	"context"
	"log/slog"

	"github.com/OWNER/aos/internal/core/collections"
	"github.com/OWNER/aos/internal/domain/collection"
)

// schemaKey extracts the id — and, for a skill-scoped declaration, the skill
// — a "collections" schema.json path names, the same way a record change is
// attributed to its collection elsewhere in this system. It reads
// collections.Natives() rather than hard-coding the pattern, so the two
// cannot drift apart.
func schemaKey(rel string) (id, skill string, ok bool) {
	for _, desc := range collections.Natives() {
		if desc.Name != "collections" {
			continue
		}
		for _, p := range desc.Patterns {
			if k, matched := p.Match(rel); matched {
				return k["id"], k["skill"], true
			}
		}
	}
	return "", "", false
}

// onSchemaChanged builds the watcher's OnSchema callback: what happens when a
// collection or view declaration appears, changes or disappears on disk
// without going through the domain's own Create — a hand-edited schema.json,
// a file restored from backup, a checkout of a different branch. Installing a
// skill already registers through collection.Service.Create and never
// reaches this path; this is the one for everything that reaches disk
// another way, which is the whole reason a watcher exists at all.
func onSchemaChanged(collSvc *collection.Service, reg *collections.Registry, pub collections.Publisher, log *slog.Logger) func(kind, rel string) {
	return func(kind, rel string) {
		ctx := context.Background()
		switch kind {
		case "collection":
			id, skill, ok := schemaKey(rel)
			if !ok {
				return
			}
			c, err := collSvc.Get(ctx, collection.GetInput{ID: id, Skill: skill})
			if err != nil {
				// Gone, or a write still in flight mid-atomic-rename; both look
				// the same from here. Unregistering a name that was never
				// registered is a no-op, so this is safe either way.
				_ = reg.Unregister(id)
				return
			}
			desc, err := collection.DescriptorFor(*c)
			if err != nil {
				log.Warn("schema.json changed but does not describe a valid collection", "id", id, "path", rel, "err", err)
				return
			}
			if err := reg.Register(desc); err != nil {
				log.Warn("could not register a collection discovered on disk", "id", id, "path", rel, "err", err)
				return
			}
			pub.Publish(ctx, collections.Changed{Collection: "collections", Key: collections.Key{"id": id}, Op: "update", Path: rel})
		case "view":
			// Views are a native collection already — nothing to register —
			// but a hand-edited view still deserves the same realtime nudge a
			// native record change gets, which schemaKind's own match routed
			// around before reaching the native publish flush.
			pub.Publish(ctx, collections.Changed{Collection: "views", Op: "update", Path: rel})
		}
	}
}

// reconcileCollections registers every collection declaration already on
// disk into reg, at boot. The watcher only reports what changes while it
// runs; a collection an earlier session created is neither a change nor
// something the watcher's initial directory walk reports — walking adds
// fsnotify watches, it does not replay the files that were already there.
// Without this, a dynamic collection from a prior session is unusable until
// its schema.json is touched again.
func reconcileCollections(ctx context.Context, collSvc *collection.Service, reg *collections.Registry, log *slog.Logger) {
	out, err := collSvc.List(ctx, collection.ListInput{})
	if err != nil {
		log.Warn("could not reconcile collections declared on disk", "err", err)
		return
	}
	for _, c := range out.Collections {
		desc, err := collection.DescriptorFor(c)
		if err != nil {
			log.Warn("a declared collection does not describe a valid schema", "id", c.ID, "err", err)
			continue
		}
		if err := reg.Register(desc); err != nil {
			log.Warn("could not register a declared collection", "id", c.ID, "err", err)
		}
	}
}
