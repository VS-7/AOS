import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from "@tanstack/react-router";
import { AosPage } from "./page";

/**
 * A page whose loader answered "not found" once is one TanStack keeps a match
 * for; going back to its address rendered the page's component with no loader
 * data while the loader ran again. Every page destructures its loader data,
 * so each revisit of a missing view or collection threw "Cannot destructure
 * property 'view' of 'route.useLoaderData(...)'" before the 404 came back.
 */
function setup(initial: string) {
  const rootRoute = createRootRoute({
    component: () => <Outlet />,
    notFoundComponent: () => <p>Page not found</p>,
  });
  const rendered: unknown[] = [];
  const thing = new AosPage("/things/$id", rootRoute, { client: {} } as never)
    .withLoader(async ({ request, response }) => {
      if (request.params.id.startsWith("nope")) return response.notFound();
      return { name: request.params.id };
    })
    .withComponent(({ route }) => {
      const data = route.useLoaderData();
      rendered.push(data);
      const { name } = data;
      return <p>thing {name}</p>;
    })
    .build();
  const home = createRoute({ path: "/", getParentRoute: () => rootRoute, component: () => <p>home</p> });
  const router = createRouter({
    routeTree: rootRoute.addChildren([home, thing]),
    history: createMemoryHistory({ initialEntries: [initial] }),
  });
  return { router, rendered };
}

afterEach(() => cleanup());

describe("AosPage", () => {
  it("never renders a page's component without its loader data", async () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      const { router, rendered } = setup("/things/nope");
      render(<RouterProvider router={router} />);
      expect(await screen.findByText("Page not found")).toBeTruthy();

      for (const to of ["/things/nope2", "/", "/things/nope", "/things/real", "/things/nope"]) {
        await act(() => router.navigate({ to }));
      }
      expect(await screen.findByText("Page not found")).toBeTruthy();

      await act(() => router.navigate({ to: "/things/real" }));
      expect(await screen.findByText("thing real")).toBeTruthy();

      expect(rendered).not.toContain(undefined);
      expect(errors).not.toHaveBeenCalled();
    } finally {
      errors.mockRestore();
    }
  });
});
