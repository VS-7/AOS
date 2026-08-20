/**
 * There is no AOS Go backend for this domain yet — AOS's own auth is
 * single-user (`internal/domain/auth.Public`, already ported as
 * `lib/auth.ts`'s `PublicUser`); this is AOS's separate multi-user
 * instance-admin surface (list/create/update/delete accounts), which AOS
 * has no equivalent for. But a real, checkable declaration for it does
 * exist — `v401/server/src/features/auth/schemas/user.schema.ts`'s Zod
 * schemas — so this is a recovered contract, not a guess from frontend
 * usage. When AOS grows a Go backend for this, re-verify against that
 * instead.
 *
 * `UserPublic` mirrors `UserPublicSchema`
 * (`UserSchema.omit({password: true, token: true})`): `username`,
 * `email`, `createdAt`, `updatedAt` are all required fields the frontend's
 * own usage never touched (`workspace/members` and `user/users` settings
 * sections only read `id`/`name`/`email`/`role`) — usage alone would have
 * missed them. `UserUpdateMeInput` mirrors
 * `UserUpdateMeBodySchema`
 * (`UserSchema.pick({name, username, email, image}).partial()`);
 * the ported profile form only ever submits `name`/`email`/`image`, but
 * `username` is a real, independently-updatable field on the wire.
 */
export type UserRole = "member" | "super";

export interface UserPublic {
  id: string;
  name: string;
  username: string;
  email: string;
  role: UserRole;
  image?: string;
  createdAt: string;
  updatedAt: string;
}

export interface UserUpdateMeInput {
  name?: string;
  username?: string;
  email?: string;
  image?: string;
}
