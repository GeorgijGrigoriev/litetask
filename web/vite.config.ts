import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const rawPort = process.env.PORT || "8080";
const backendPort = rawPort.startsWith(":") ? rawPort.slice(1) : rawPort;

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": `http://localhost:${backendPort}`,
    },
  },
});
