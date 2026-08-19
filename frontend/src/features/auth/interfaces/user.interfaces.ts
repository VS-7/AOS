/**
 * Reconstructed from usage: the file was type-only and the bundler erased
 * it. There is no Go backend for this domain yet — AOS's own auth is
 * single-user (`internal/domain/auth.Public`, already ported as
 * `lib/auth.ts`'s `PublicUser`); this is Fractal's separate multi-user
 * instance-admin surface (list/create/update/delete accounts), which AOS
 * has no equivalent for. When it does, this contract becomes verifiable
 * and should be checked against it.
 *
 * Usage sites (v401/web/src): the workspace members section and the
 * instance users section (`features/workspace/presentation/components/
 * settings/.../sections/{workspace/members,user/users}/index.tsx`) read
 * `id`, `name`, `email`, `role` ("member" | "super"); the profile section
 * (`.../sections/user/profile/index.tsx`) also reads `image` and calls
 * `updateProfile({ name, email, image })`.
 */
export type FractalUserRole = "member" | "super";

export interface FractalUserPublic {
  id: string;
  name: string;
  email: string;
  image?: string;
  role: FractalUserRole;
}

export interface FractalUserUpdateMeInput {
  name?: string;
  email?: string;
  image?: string;
}
