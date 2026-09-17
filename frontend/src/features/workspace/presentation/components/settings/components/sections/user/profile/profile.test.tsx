import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

/**
 * The profile section's defects were all about *when* and *with what* it
 * talks to the daemon, so the daemon is the mock here: a name that must not
 * be rewritten under the person typing it, a message beside the field that
 * says why nothing was saved, and a language change that saves before it
 * remounts the tree.
 */

const daemon = vi.hoisted(() => ({
  user: { id: "u1", name: "Vitor Sergio", email: "vitor@example.test", image: "" },
  calls: [] as string[],
  updateProfile: vi.fn(),
  saveConfigSettings: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));

vi.mock("@/components/ui/image-upload", () => ({
  ImageUpload: () => <div data-testid="image-upload" />,
}));

// Radix's select needs pointer geometry jsdom does not have; what matters
// here is that picking a language runs the section's own handler.
vi.mock("@/components/ui/select", () => ({
  Select: ({ value, onValueChange, disabled, children }: any) => (
    <select
      data-testid="language"
      value={value}
      disabled={disabled}
      onChange={(event) => onValueChange(event.target.value)}
    >
      {children}
    </select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: ({ children }: any) => <>{children}</>,
  SelectItem: ({ value, children }: any) => <option value={value}>{children}</option>,
}));

vi.mock("../../../../helpers/save-config-settings", () => ({
  saveConfigSettings: (set: Record<string, unknown>) => {
    daemon.calls.push(`config:${Object.keys(set).join(",")}`);
    return daemon.saveConfigSettings(set);
  },
}));

// The real aos.useForm (app/builders/app.tsx): react-hook-form with a
// `submit` that runs `onSubmit` and resets the form to what it returned.
vi.mock("@/app/aos", () => ({
  aos: {
    useForm: ({ schema, values, onSubmit }: any) => {
      const form = useForm({ resolver: zodResolver(schema), defaultValues: values, mode: "onChange" });
      return Object.assign(form, {
        isLoading: false,
        submit: form.handleSubmit(async (data: any) => {
          const next = await onSubmit(data);
          form.reset(next ?? data);
        }),
      });
    },
    stores: {
      auth: {
        useState: (select: (s: any) => unknown) => select({ user: daemon.user }),
        get state() {
          return { user: daemon.user };
        },
        actions: {
          updateProfile: (input: unknown) => {
            daemon.calls.push("profile");
            return daemon.updateProfile(input);
          },
          updatePassword: vi.fn(),
        },
      },
      config: {
        useState: () => ({ region: { language: "en-US", timezone: "UTC", city: "", country: "" } }),
        state: { region: { language: "en-US", timezone: "UTC", city: "", country: "" } },
      },
    },
  },
}));

import { UserProfileSection } from "./index";

afterEach(() => {
  cleanup();
  daemon.calls = [];
  daemon.updateProfile.mockReset().mockResolvedValue({ error: undefined });
  daemon.saveConfigSettings.mockReset().mockResolvedValue(true);
});

describe("the profile section", () => {
  it("keeps the spaces somebody is still typing in their name", async () => {
    daemon.updateProfile.mockResolvedValue({ error: undefined });
    render(<UserProfileSection />);

    const name = screen.getByDisplayValue("Vitor Sergio") as HTMLInputElement;
    fireEvent.change(name, { target: { value: "Vitor Sergio " } });
    fireEvent.blur(name);

    // The daemon is handed the trimmed name; the field keeps what was typed,
    // because the schema judges the value without rewriting it.
    await waitFor(() => expect(name.value).toBe("Vitor Sergio "));
  });

  it("says beside the field why a name of spaces is not saved", async () => {
    render(<UserProfileSection />);

    const name = screen.getByDisplayValue("Vitor Sergio") as HTMLInputElement;
    fireEvent.change(name, { target: { value: "  " } });

    expect(await screen.findByText("Name must be at least 2 characters.")).toBeTruthy();
    expect(daemon.calls).not.toContain("profile");
  });

  // #190: switching first and leaving the save to the autosave lost it —
  // changing language remounts the tree and takes the pending save with it.
  it("saves before it switches the language", async () => {
    render(<UserProfileSection />);

    const name = screen.getByDisplayValue("Vitor Sergio") as HTMLInputElement;
    fireEvent.change(name, { target: { value: "Vitor S." } });
    await act(async () => {
      fireEvent.change(screen.getByTestId("language"), { target: { value: "pt-BR" } });
    });

    await waitFor(() => expect(daemon.saveConfigSettings).toHaveBeenCalled());
    // The edit reaches the account first; the language is the last thing
    // written, because writing it is what remounts the tree.
    expect(daemon.calls.indexOf("profile")).toBeLessThan(daemon.calls.indexOf("config:region.language"));
    expect(daemon.calls.at(-1)).toBe("config:region.language");
    expect(daemon.saveConfigSettings).toHaveBeenCalledWith({ "region.language": "pt-BR" });
  });
});
