import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// The bridge is never there in a test, so every call falls through to HTTP —
// which is the transport these assertions are about.
vi.mock("@wailsio/runtime", () => ({
  Call: { ByName: () => Promise.reject(new Error("no wails host")) },
}));

import { changePassword, updateProfile, session, PublicUser } from "./auth";

type Recorded = { url: string; init: RequestInit | undefined };

let calls: Recorded[] = [];

function answer(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

beforeEach(() => {
  calls = [];
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function stubFetch(body: unknown, status = 200) {
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string, init?: RequestInit) => {
      calls.push({ url: String(url), init });
      return Promise.resolve(answer(body, status));
    }),
  );
}

describe("changePassword", () => {
  // The defect: this sent `{current, next}` and the handler
  // (internal/transport/authapi) decodes `currentPassword`/`newPassword`. Go
  // decoded two empty strings, so the current-password check failed every
  // time and the screen told people their correct password was wrong.
  it("sends the field names the daemon decodes", async () => {
    stubFetch({ data: {} });

    await changePassword("velha", "nova");

    expect(calls).toHaveLength(1);
    expect(calls[0].url).toContain("/api/auth/password");
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({
      currentPassword: "velha",
      newPassword: "nova",
    });
  });
});

describe("updateProfile", () => {
  it("posts the name and email and returns the account", async () => {
    const user: PublicUser = {
      id: "u-1",
      name: "Vitor Sérgio",
      username: "vitor",
      email: "vitor@example.test",
      role: "super",
    };
    stubFetch({ data: { user } });

    const result = await updateProfile("Vitor Sérgio", "vitor@example.test");

    expect(calls[0].url).toContain("/api/auth/profile");
    expect(calls[0].init?.method).toBe("POST");
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({
      name: "Vitor Sérgio",
      email: "vitor@example.test",
    });
    expect(result.user).toEqual(user);
  });

  it("surfaces a refusal rather than resolving with nothing", async () => {
    stubFetch({ error: { code: "AOS_AUTH_NAME_REQUIRED", message: "an account needs a name" } }, 400);

    await expect(updateProfile("", "vitor@example.test")).rejects.toMatchObject({
      code: "AOS_AUTH_NAME_REQUIRED",
    });
  });
});

describe("session", () => {
  // Every caller writes `const { user } = await session()`. Both transports
  // have to hand back that shape — the bridge one used to answer a bare user,
  // which is what left the desktop window with no name and no `super` role.
  it("answers an object with a user on it", async () => {
    stubFetch({
      data: { user: { id: "u-1", name: "Vitor", username: "vitor", email: "v@e.test", role: "super" } },
    });

    const { user } = await session();

    expect(user.name).toBe("Vitor");
    expect(user.role).toBe("super");
  });
});
