import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // `@/` is for src/ modules shared across pages; imports inside a page stay relative.
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  server: {
    // `pnpm dev` talks to the gateway started by `make dev-native`.
    proxy: { "/api": "http://localhost:3000" },
  },
  test: { environment: "node" },
});
