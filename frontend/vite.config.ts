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
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  // Monaco ships from node_modules, in the bundle, rather than served from
  // the daemon the way the original does — see docs/05 - Transporte/Artifacts
  // e Estáticos.md's "Monaco no bundle" decision. Vite's dependency
  // pre-bundler otherwise chokes on Monaco's internal ESM structure.
  optimizeDeps: {
    include: ["monaco-editor"],
  },
  // The environment is node, not jsdom: what we're testing is the facade —
  // name translation, payload assembly, and envelope shape. None of that
  // touches the DOM, and a jsdom here would only add startup time to every run.
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
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
    },
  },
});
