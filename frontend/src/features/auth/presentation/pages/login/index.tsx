import * as React from "react";
import { aos } from "@/app/aos";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { Logo } from "@/components/ui/logo";
import { Page, PageBody } from "@/components/ui/page";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "sonner";
import { LogInIcon } from "lucide-react";
import { useRouter } from "@tanstack/react-router";

export const LoginPage = aos.page("/login")
  .withMetadata({
    title: "Login — Fractal",
    description: "Authenticate to access your Fractal instance",
  })
  .use(WorkspacePageMiddleware())
  .withComponent(({ route }) => {
    const router = useRouter();

    const [email, setEmail] = React.useState("");
    const [password, setPassword] = React.useState("");
    const [isLoading, setIsLoading] = React.useState(false);

    const handleSubmit = async (e: React.FormEvent) => {
      e.preventDefault();

      if (!email.trim()) {
        toast.error("Please enter your email.");
        return;
      }

      if (!password.trim()) {
        toast.error("Please enter your password.");
        return;
      }

      setIsLoading(true);

      try {
        const result = await aos.stores.auth.actions.login({
          email: email.trim(),
          password,
        });

        if (result.error) {
          toast.error(
            result.error.message || "Invalid credentials. Please try again.",
          );
          return;
        }

        toast.success("Authenticated successfully!");
        router.navigate({ to: "/" });
      } catch (error: unknown) {
        const message =
          error instanceof Error
            ? error.message
            : "Authentication failed. Please try again.";
        toast.error(message);
      } finally {
        setIsLoading(false);
      }
    };

    return (
      <Page className="h-screen">
        <PageBody className="h-screen">
          <div className="flex h-screen w-full flex-col items-center justify-center p-8">
            <div className="flex w-full max-w-[400px] flex-col items-center gap-8 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-full text-primary-foreground shadow-sm">
                <Logo className="h-10 w-10" />
              </div>

              <h1 className="text-3xl font-bold tracking-tight">
                Log in to Fractal
              </h1>

              <form
                onSubmit={handleSubmit}
                className="flex w-full flex-col gap-4"
              >
                <Input
                  type="email"
                  placeholder="Email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={isLoading}
                  className="text-center text-base h-11 rounded-full"
                  autoFocus
                  autoComplete="email"
                />
                <Input
                  type="password"
                  placeholder="Password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={isLoading}
                  className="text-center text-base h-11 rounded-full"
                  autoComplete="current-password"
                />
                <Button
                  type="submit"
                  disabled={
                    isLoading || !email.trim() || !password.trim()
                  }
                  className="h-11 rounded-full"
                >
                  {isLoading ? "Signing in..." : "Sign In"}
                </Button>
              </form>

              <p className="text-xs text-muted-foreground">
                © Nubler Digital LTDA
              </p>
            </div>
          </div>
        </PageBody>
      </Page>
    );
  })
  .build();
