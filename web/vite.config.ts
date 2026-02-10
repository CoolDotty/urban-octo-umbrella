import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8090",
      "/auth": "http://localhost:8090",
      "/_/": "http://localhost:8090",
    },
  },
  build: {
    outDir: "../server/web/dist",
    emptyOutDir: true,
  },
});
