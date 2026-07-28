import { fileURLToPath, URL } from "node:url";
import vue from "@vitejs/plugin-vue";
import { defineConfig } from "vite";

export default defineConfig({
  root: fileURLToPath(new URL("./web", import.meta.url)),
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./web/src", import.meta.url)),
    },
  },
  build: {
    outDir: fileURLToPath(new URL("./internal/web/dist", import.meta.url)),
    emptyOutDir: true,
    assetsDir: "assets",
    sourcemap: false,
  },
  server: {
    host: "0.0.0.0",
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:8090",
      "/ws": {
        target: "ws://127.0.0.1:8090",
        ws: true,
      },
    },
  },
});
