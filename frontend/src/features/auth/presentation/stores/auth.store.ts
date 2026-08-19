import type { PublicUser } from "@/lib/auth";

/**
 * AOS's `auth` feature is real (see `features/auth/{LoginPage,AuthGate,
 * OnboardingForm}.tsx`), just not built as a `presentation/stores/*` module
 * the way Fractal's was — that pattern belongs to the store system Task 10
 * wires up. `task`'s assignee.helper.ts only needs the current user's
 * public shape, which AOS already has for real as `PublicUser`
 * (`lib/auth.ts`, mirroring the Go side's `internal/domain/auth.Public`).
 * `image` is the one field Fractal's assignee UI reads that AOS's user has
 * no equivalent for yet, so it stays optional and unset.
 */
export type AuthSelfProfile = PublicUser & { image?: string };
