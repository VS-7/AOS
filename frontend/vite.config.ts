/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { fileURLToPath, URL } from "node:url";

// The bundle is served two ways: by the daemon over HTTP, and from inside the
// desktop binary as an embedded filesystem. Neither serves it from a
// subdirectory, so the base is the root and the output goes where the Go embed
// directive looks for it.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: "/",
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),

      // The icon set, resolved to the barrel instead of the pre-minified
      // bundle its own exports map picks for a production build.
      //
      // @hugeicons/core-free-icons publishes two entry points behind the
      // `development`/`production` export conditions: `index.js`, a 498 kB
      // barrel that re-exports one file per icon, and `index.min.js`, a
      // single 4.7 MB module with every icon inlined into one scope. Rollup
      // can tree-shake the first and cannot tree-shake the second, and a
      // production build resolves to the second — so all 4,000-odd icons
      // ended up in the startup bundle, a fifth of the whole application, to
      // draw the forty this interface actually names.
      //
      // Both entry points export the same names, so nothing else changes.
      "@hugeicons/core-free-icons": fileURLToPath(
        new URL("./node_modules/@hugeicons/core-free-icons/dist/esm/index.js", import.meta.url),
      ),
    },
  },
  // Monaco ships from node_modules, in the bundle, rather than served from
  // the daemon the way the original does — see docs/05 - Transporte/Artifacts
  // e Estáticos.md's "Monaco no bundle" decision. Vite's dependency
  // pre-bundler otherwise chokes on Monaco's internal ESM structure.
  optimizeDeps: {
    include: ["monaco-editor"],
  },
  // jsdom, not node (final review, second pass): `lib/aos-facade.test.ts`'s
  // "useQuery's queryFn" suite renders the real `useQuery`/`useMutation`
  // hooks (via `@testing-library/react`'s `renderHook`) to test them as
  // what they are — React hooks, one of which (`ActionNode.useQuery`) runs
  // a real `React.useEffect` for the `onSuccess` shim. Calling a hook's
  // body directly outside a render (this suite's original approach) broke
  // the moment that `useEffect` landed (Task 9's bulk copy, `6ab2f34`) —
  // React's dispatcher only exists during an actual render, real or
  // `renderHook`'s. The rest of the suite (name translation, payload
  // assembly, envelope shape) touches no DOM and pays jsdom's startup cost
  // for no benefit, but it's one environment per file, not per test, and
  // this project has exactly one test file that needs it.
  test: {
    environment: "jsdom",
    // The Wails runtime is replaced wholesale in the suite — see
    // src/test/wails-runtime-stub.ts for the import side effect that makes
    // the real module unusable in a test process, and how a test that needs
    // it to behave a particular way still says so for itself.
    alias: {
      "@wailsio/runtime": fileURLToPath(new URL("./src/test/wails-runtime-stub.ts", import.meta.url)),
    },
    // `.tsx`, not just `.ts`: `lib/aos-facade.test.tsx` renders real hooks
    // through a `QueryClientProvider`, which needs JSX — see that file's
    // own comment and `vite.config.ts`'s `test.environment` comment above.
    include: ["src/**/*.test.{ts,tsx}"],
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    // A source map is what makes a stack trace from a user's machine readable.
    // It costs bundle size on disk and nothing at runtime.
    sourcemap: true,
  },
  server: {
    // wails3 dev drives this through `--port`/`--strictPort` (see the root
    // Taskfile's VITE_PORT var), which already overrides this default; the
    // env var is read too so a bare `vite`/`wails3 dev` invocation without
    // that flag still lands on the same port instead of a random one.
    port: Number(process.env.WAILS_VITE_PORT) || 5327,
    strictPort: true,
    proxy: {
      // In development the page is served by Vite and the daemon answers on its
      // own port. In production both come from the same origin, so this proxy
      // is the one thing that exists only while developing.
      "/api": { target: "http://127.0.0.1:5326", changeOrigin: true },
      "/ws": { target: "ws://127.0.0.1:5326", ws: true },
      // Artifacts (internal/transport/artifactapi) — missed when that mount
      // point was added, which meant every artifact URL a dev build opened
      // resolved to Vite's own index.html (a 200, wrong content) instead of
      // ever reaching the daemon.
      "/v": { target: "http://127.0.0.1:5326", changeOrigin: true },
    },
  },
});
