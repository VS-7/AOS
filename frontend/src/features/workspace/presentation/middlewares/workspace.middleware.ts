import { aos } from "@/app/aos";

export const WorkspacePageMiddleware = aos
  .middleware()
  .withLoader(async ({ client, context, request, stores, response }) => {
    console.log("[WorkspaceMiddleware] BEGIN", {
      url: request.url,
      isOnLoginPage: request.url.includes("/login"),
      isOnOnboarding: request.url.includes("/onboarding"),
      authState: stores.auth.state,
    });

    // ── Auth Check ──────────────────────────────────────────
    const isOnLoginPage = request.url.includes("/login");

    const { isAuthenticated } = stores.auth.state;

    if (!isAuthenticated && !isOnLoginPage) {
      const onboarding = stores.auth.state.onboarding;
      console.log(
        "[WorkspaceMiddleware] Auth enabled + not authenticated + not login page → onboarding status:",
        onboarding,
      );

      if (onboarding === "waiting") {
        console.log(
          "[WorkspaceMiddleware] Onboarding waiting → redirect to /onboarding",
        );
        return response.redirect("/onboarding");
      }

      console.log("[WorkspaceMiddleware] Onboarding done → redirect to /login");
      return response.redirect("/login");
    }

    // If authenticated and on login page, redirect to home
    if (isAuthenticated && isOnLoginPage) {
      console.log(
        "[WorkspaceMiddleware] Authenticated + on login page → redirect to /",
      );
      return response.redirect("/");
    }

    // ── Workspace Check ────────────────────────────────────
    const isEditingOnboarding = request.url.includes("/onboarding");
    const onboarding = stores.auth.state.onboarding;

    if (onboarding === "waiting" && !isEditingOnboarding) {
      console.log(
        "[WorkspaceMiddleware] Onboarding waiting + not on onboarding → redirect to /onboarding",
      );
      return response.redirect("/onboarding");
    }

    if (onboarding === "done" && isEditingOnboarding) {
      console.log(
        "[WorkspaceMiddleware] Onboarding done + on onboarding → redirect to /",
      );
      return response.redirect("/");
    }

    console.log("[WorkspaceMiddleware] All checks passed, proceeding.");
    return {};
  })
  .build();
