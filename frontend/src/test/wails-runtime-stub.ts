/**
 * `@wailsio/runtime`, inert, for tests.
 *
 * The real module is not importable in a test process without leaving
 * something behind. `dist/drag.js` starts a `setInterval` at import time that
 * polls every 50 ms for five seconds looking for a Wails environment, and only
 * clears itself once it finds one — which never happens outside a real window.
 * Vitest gives each test file its own jsdom and tears it down when the file
 * ends; a tick that lands after that teardown dereferences a `window` that no
 * longer exists, and the run fails with
 *
 *     ReferenceError: window is not defined
 *       ❯ Timeout._onTimeout node_modules/@wailsio/runtime/dist/drag.js:69:13
 *
 * outside any test, after all of them have passed. It is a race — the interval
 * has to tick in the gap between one file's teardown and the run ending — so
 * it appears and disappears with machine speed. It surfaced the day CI started
 * running these tests at all.
 *
 * Aliased in `vite.config.ts` under `test.alias`, so this replaces the module
 * everywhere in the suite. A test that needs the runtime to behave a
 * particular way still declares its own `vi.mock("@wailsio/runtime", …)`,
 * which takes precedence over this; what this removes is the import side
 * effect for every test that only reaches the module incidentally, through
 * `lib/client.ts` or `lib/wails.ts`, and never touches it.
 */

const rejects = () => Promise.reject(new Error("@wailsio/runtime is stubbed in tests"));
const resolves = () => Promise.resolve();

export const Call = { ByName: rejects, ByID: rejects };

export const Browser = { OpenURL: resolves };

export const Clipboard = { SetText: resolves, Text: () => Promise.resolve("") };

export const Dialogs = {
  Info: resolves,
  Warning: resolves,
  Error: resolves,
  Question: () => Promise.resolve(""),
  OpenFile: () => Promise.resolve([]),
  SaveFile: () => Promise.resolve(""),
};

// False across the board: a test process is not a desktop window, and the
// interface's own `lib/wails.ts` falls back to the declared platform when
// every one of these says no.
export const System = {
  IsMac: () => false,
  IsWindows: () => false,
  IsLinux: () => false,
  IsDesktop: () => false,
  Environment: () => Promise.resolve({}),
};

export const Window = {
  Minimise: resolves,
  Maximise: resolves,
  UnMaximise: resolves,
  ToggleMaximise: resolves,
  Close: resolves,
  IsMaximised: () => Promise.resolve(false),
  Reload: resolves,
  SetTitle: resolves,
};

export const Events = {
  On: () => () => {},
  Off: () => {},
  Emit: resolves,
  Types: { Common: {} },
};

export const Screens = { GetAll: () => Promise.resolve([]), GetCurrent: () => Promise.resolve(null) };

export const Application = { Quit: resolves, Hide: resolves, Show: resolves };

export const WML = { Reload: () => {} };
