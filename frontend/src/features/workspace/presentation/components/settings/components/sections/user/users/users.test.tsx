import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import * as React from "react";

const state = vi.hoisted(() => ({ data: undefined as unknown, isLoading: false }));

vi.mock("@/lib/aos-facade", () => ({
  api: {
    user: {
      list: { useQuery: () => ({ data: state.data, isLoading: state.isLoading, refetch: vi.fn() }) },
    },
  },
}));

import { UserUsersSection } from "./index";

beforeEach(() => {
  cleanup();
  state.data = {
    users: [
      { id: "u1", name: "Vitor Sergio", username: "vitor", email: "vitor@example.test", role: "super" },
      { id: "u2", name: "Luara", username: "luara", email: "luara@example.test", role: "member" },
    ],
  };
});

// The whole section sat behind a "Domain not available yet — the Go backend
// does not publish this domain" panel, although the roster is readable, and
// the roster itself read the list answer as an array when it is {users}.
describe("Settings > Users", () => {
  it("lists the accounts of this installation", () => {
    render(React.createElement(UserUsersSection));
    expect(screen.getByText("Vitor Sergio")).toBeTruthy();
    expect(screen.getByText("luara@example.test")).toBeTruthy();
  });

  it("offers no action that cannot be carried out, and says why", () => {
    render(React.createElement(UserUsersSection));
    expect(screen.queryByRole("button", { name: /add user/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /delete/i })).toBeNull();
    expect(screen.getByText(/cannot be added, changed or removed from here/i)).toBeTruthy();
  });
});
