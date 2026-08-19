/**
 * Reconstructed from usage: the file was type-only and the bundler erased
 * it. There is no Go backend for this domain yet — when there is, this
 * contract becomes verifiable and should be checked against it.
 *
 * Usage sites (v401/web/src): `features/activity/presentation/stores/
 * activity.store.ts` reads `.read.length` on each entry (so `read` is an
 * array, not a boolean, despite `inbox-notification-item.component.tsx`
 * only ever testing it for truthiness); the inbox panel and its row
 * component read `id`, `title`, `body`, `icon`, `createdAt`, and
 * `actions[].to`/`.label`.
 */
export interface NotificationAction {
  to: string;
  label: string;
}

export interface NotificationPayload {
  id: string;
  title: string;
  body?: string;
  icon?: string;
  actions?: NotificationAction[];
  /** User/agent ids that have read this entry — empty means unread. */
  read: string[];
  createdAt: string;
}
