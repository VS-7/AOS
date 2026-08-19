/**
 * Reconstructed from usage — but not merely from the frontend's own usage.
 * The file was type-only and the bundler erased it from every extraction
 * (including the frontend's own `../interfaces` re-export in
 * `v401/server/src/@core/builders/notification/index.ts` — even the
 * server never got its interface file back). There is no Go backend for
 * this domain yet — when there is, this contract becomes verifiable and
 * should be checked against it.
 *
 * `NotificationPayload` is the wire shape the server's notification
 * builder constructs and the frontend receives verbatim, so the
 * authoritative source is the server's own construction site, not the
 * frontend's (weaker) usage:
 * `v401/server/src/@core/builders/notification/builders/events.builder.ts:101-113`
 * (the literal object that becomes every notification) and
 * `.../builders/manager.builder.ts:235-249` +
 * `.../adapters/bun/fs.adapter.ts:50-124` (read/write sites proving
 * `read`'s real shape and which fields are read without an `undefined`
 * guard, i.e. always present).
 *
 * `read: string[]` was my first pass, from the frontend's own
 * `activity.store.ts` (`.read.length === 0`) and
 * `inbox-notification-item.component.tsx` (`!read`) — both are true of a
 * plain array OR an array of per-user read receipts, so frontend usage
 * alone cannot tell the two apart. The server's construction/mutation
 * sites can: `manager.builder.ts:238` does
 * `notification.read.some((r) => r.causer === options.causer)` and
 * `:240-243` pushes `{ causer, readAt: new Date().toISOString() }` —
 * `read` is a receipt array, not a bare id list. Corrected below, along
 * with three more fields the same construction site showed were wrong or
 * missing: `namespace`/`event`/`data` are always present (server code
 * reads `n.namespace`, `n.event` with no optional-chaining anywhere in
 * `fs.adapter.ts`, and `data: resolvedData` is a non-optional
 * `Record<string, unknown>` at construction), and `body` is never
 * optional (`FractalTemplate.resolve` — `helpers/template.ts` — always
 * returns `string`, never `undefined`).
 */
export interface NotificationAction {
  to: string;
  label: string;
}

/** One user's read receipt for a notification (`manager.builder.ts`'s `markAsRead`). */
export interface NotificationReadReceipt {
  causer: string;
  readAt: string;
}

/**
 * Resolved per-event delivery settings, snapshotted onto the payload at
 * creation (`FractalNotificationDefinitionHelper.resolve_settings`).
 * Optional on read: `manager.builder.ts`'s `_resolve_payload_settings`
 * guards it with `!== undefined`, implying older persisted records can
 * lack it even though every notification created today always has one.
 */
export interface NotificationSettings {
  notify: boolean;
  routine: boolean;
}

export interface NotificationPayload {
  id: string;
  namespace: string;
  event: string;
  title: string;
  body: string;
  icon?: string;
  actions?: NotificationAction[];
  /** Structured payload the template placeholders were resolved from. */
  data: Record<string, unknown>;
  /** Per-user read receipts — empty means nobody has read it yet. */
  read: NotificationReadReceipt[];
  createdAt: string;
  settings?: NotificationSettings;
}
