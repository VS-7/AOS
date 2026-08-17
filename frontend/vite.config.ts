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
  build: {
    outDir: "dist",
    emptyOutDir: true,
    // A source map is what makes a stack trace from a user's machine readable.
    // It costs bundle size on disk and nothing at runtime.
    sourcemap: true,
  },
  server: {
    port: 5327,
    proxy: {
      // In development the page is served by Vite and the daemon answers on its
      // own port. In production both come from the same origin, so this proxy
      // is the one thing that exists only while developing.
      "/api": { target: "http://127.0.0.1:5326", changeOrigin: true },
      "/ws": { target: "ws://127.0.0.1:5326", ws: true },
    },
  },
});
