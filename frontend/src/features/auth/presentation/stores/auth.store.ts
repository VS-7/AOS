import { AosStore } from "@/app/builders/store";
import { api } from "@/lib/aos-facade";
import type {
  UserPublic,
  UserUpdateMeInput,
} from "@/features/auth/interfaces/user.interfaces";
import type { AuthChangePassword } from "@/features/auth/interfaces/auth.interfaces";

const setAuthCookie = (token: string | undefined) => {
  if (typeof window !== "undefined") {
    if (token) {
      document.cookie = `sessionToken=${encodeURIComponent(token)}; path=/; max-age=86400; SameSite=Lax`;
    } else {
      document.cookie = `sessionToken=; path=/; max-age=0; SameSite=Lax`;
    }
  }
};

export type AuthSelfProfile = UserPublic & {
  hasToken: boolean;
  tokenMasked: string | null;
};

export interface AuthState {
  token: string | null;
  isAuthenticated: boolean;
  /** Installation setup status from `config.get().status`. */
  onboarding: "done" | "waiting";
  user: AuthSelfProfile | null;
}

async function fetchSelfProfile(): Promise<AuthSelfProfile | null> {
  const session = await api.session.get.query().catch(() => null);
  if (session?.data) {
    return session.data as AuthSelfProfile;
  }

  return null;
}

/**
 * Resolves installation status from the public `auth.getStatus` probe.
 *
 * Must not call authenticated endpoints (e.g. `config.get`) — unauthenticated
 * first visits need onboarding vs login routing without a session token.
 */
async function fetchInstallationStatus(): Promise<"done" | "waiting"> {
  const response = await api.auth.getStatus.query().catch(() => null);
  const onboarding = response?.data?.onboarding;
  return onboarding === "done" || onboarding === "waiting"
    ? onboarding
    : "waiting";
}

/**
 * Whether the browser still has a usable session cookie / persisted token.
 *
 * Used with `auth.me` — session.get never throws; the controller returns 401
 * when anonymous.
 */
async function probeAuthenticated(): Promise<boolean> {
  const profile = await fetchSelfProfile();
  return profile !== null;
}

export const AuthStore = AosStore.create("auth")
  .withState<AuthState>({
    token: null,
    isAuthenticated: false,
    onboarding: "waiting",
    user: null,
  })
  .withPersistence({
    enabled: true,
  })
  .withPreload(async (ctx) => {
    try {
      const onboarding = await fetchInstallationStatus();
      const isAuthenticated = await probeAuthenticated();
      const currentState = ctx.state.get();
      const token = isAuthenticated ? currentState.token : null;

      if (!isAuthenticated) {
        setAuthCookie(undefined);

        return {
          token: null,
          isAuthenticated: false,
          onboarding,
          user: null,
        };
      }

      const user = await fetchSelfProfile();

      return {
        token,
        isAuthenticated: true,
        onboarding,
        user,
      };
    } catch {
      setAuthCookie(undefined);

      return {
        token: null,
        isAuthenticated: false,
        onboarding: "waiting" as const,
        user: null,
      };
    }
  })
  .addAction("login", (ctx) => async (params: { email: string; password: string }) => {
    try {
      const response = await api.auth.login.mutate({ body: params });

      if (!response.data) {
        return { success: false, error: new Error("No response data") };
      }

      const { token } = response.data;
      setAuthCookie(token);

      const user = await fetchSelfProfile();
      const onboarding = await fetchInstallationStatus();

      ctx.state.set({
        token,
        isAuthenticated: true,
        onboarding,
        user,
      });

      return { success: true, error: null };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error : new Error(String(error)),
      };
    }
  })
  .addAction("logout", (ctx) => async () => {
    try {
      await api.auth.logout.mutate();
    } catch {
      // Best-effort server logout — always clear local session state.
    }

    setAuthCookie(undefined);

    ctx.state.set({
      token: null,
      isAuthenticated: false,
      onboarding: ctx.state.get().onboarding,
      user: null,
    });
  })
  .addAction("updateProfile", (ctx) => async (params: UserUpdateMeInput) => {
    try {
      const response = await api.session.updateProfile.mutate({ body: params as Record<string, unknown> });

      if (!response.data) {
        return { success: false, error: new Error("No response data") };
      }

      ctx.state.set({
        ...ctx.state.get(),
        user: response.data as AuthSelfProfile,
      });

      return { success: true, error: null };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error : new Error(String(error)),
      };
    }
  })
  .addAction("updatePassword", () => async (params: AuthChangePassword) => {
    try {
      await api.password.change.mutate({ body: params });
      return { success: true, error: null };
    } catch (error) {
      return {
        success: false,
        error: error instanceof Error ? error : new Error(String(error)),
      };
    }
  })
  .addAction("regenerateToken", () => async () => {
    try {
      const response = await api.token.regenerate.mutate({ body: {} });

      if (!response.data?.token) {
        return {
          success: false,
          token: null,
          error: new Error("No token returned"),
        };
      }

      return {
        success: true,
        token: response.data.token,
        error: null,
      };
    } catch (error) {
      return {
        success: false,
        token: null,
        error: error instanceof Error ? error : new Error(String(error)),
      };
    }
  })
  .addAction("refreshUser", (ctx) => async () => {
    const user = await fetchSelfProfile();

    ctx.state.set({
      ...ctx.state.get(),
      user,
    });

    return user;
  })
  .addAction("clear", (ctx) => async () => {
    setAuthCookie(undefined);

    ctx.state.set({
      token: null,
      isAuthenticated: false,
      onboarding: ctx.state.get().onboarding,
      user: null,
    });
  })
  .build();
