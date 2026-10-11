import { resolve } from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Two static entry pages (home and docs), so the build needs no client-side
// routing and deploys as-is to Vercel or any static host.
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      input: {
        home: resolve(__dirname, "index.html"),
        docs: resolve(__dirname, "docs.html"),
      },
    },
  },
});
