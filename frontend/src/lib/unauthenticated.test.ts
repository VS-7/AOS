import { describe, expect, it, vi } from "vitest";

vi.mock("@wailsio/runtime", () => ({ Call: { ByName: vi.fn() } }));

const { UNAUTHENTICATED_EVENT, unwrap } = await import("./client");

/**
 * A session lasts thirty days, and can also be revoked — a logout from
 * another window, a daemon whose accounts were reset. There was no path back
 * from either: `AuthGate` asks once, at mount, so the application stayed on
 * its screens and answered every action with "this request carries no valid
 * credential" as a toast. The only ways out were a reload or a restart.
 *
 * The transports announce it now, and AuthGate re-asks.
 */
describe("an envelope that says the credential is no good", () => {
  it("announces it to the page", () => {
    const heard = vi.fn();
    window.addEventListener(UNAUTHENTICATED_EVENT, heard);

    expect(() =>
      unwrap({ error: { code: "AOS_HTTP_UNAUTHENTICATED", message: "no credential" } }),
    ).toThrow();

    expect(heard).toHaveBeenCalledTimes(1);
    window.removeEventListener(UNAUTHENTICATED_EVENT, heard);
  });

  it("announces a bare 401 too, whatever the code says", () => {
    const heard = vi.fn();
    window.addEventListener(UNAUTHENTICATED_EVENT, heard);

    expect(() => unwrap({ error: { code: "SOMETHING_ELSE", status: 401 } })).toThrow();

    expect(heard).toHaveBeenCalledTimes(1);
    window.removeEventListener(UNAUTHENTICATED_EVENT, heard);
  });

  it("stays quiet for every other refusal", () => {
    const heard = vi.fn();
    window.addEventListener(UNAUTHENTICATED_EVENT, heard);

    expect(() =>
      unwrap({ error: { code: "AOS_TASK_STATUS_NOT_WRITABLE", status: 409 } }),
    ).toThrow();
    expect(() => unwrap({ data: { fine: true } })).not.toThrow();

    expect(heard).not.toHaveBeenCalled();
    window.removeEventListener(UNAUTHENTICATED_EVENT, heard);
  });
});
