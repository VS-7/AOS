import { describe, expect, it } from "vitest";

import { toSubmitError } from "./form-error";

describe("toSubmitError", () => {
  it("passes a real Error through untouched", () => {
    const error = Object.assign(new Error("the daemon said no"), { code: "AOS_NOPE" });
    expect(toSubmitError(error)).toBe(error);
  });

  // The stores' actions answer `{ error: { message } }`, and a form's
  // onSubmit rethrows that object. `new Error(String(it))` made the toast
  // read "[object Object]" — the password form's whole error message.
  it("keeps the message of a thrown { message } object", () => {
    const error = toSubmitError({ message: "Current password is wrong", code: "AUTH_BAD_PASSWORD" });
    expect(error).toBeInstanceOf(Error);
    expect(error.message).toBe("Current password is wrong");
    expect((error as Error & { code?: string }).code).toBe("AUTH_BAD_PASSWORD");
  });

  it("still says something for anything else", () => {
    expect(toSubmitError("offline").message).toBe("offline");
    expect(toSubmitError(undefined).message).toBe("undefined");
  });
});
