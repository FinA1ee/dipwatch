import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // `pnpm dev` talks to the gateway started by `make dev-native`.
    proxy: { "/api": "http://localhost:3000" },
  },
});
