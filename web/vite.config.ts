/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import viteReact from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import { writeFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
      routesDirectory: "./src/app/routes",
      generatedRouteTree: "./src/app/routeTree.gen.ts",
    }),
    viteReact(),
    tailwindcss(),
    {
      // web/dist is embedded by mesa-server (web/embed.go); the tracked .gitkeep keeps the
      // directory in a fresh checkout, and every build puts it back after emptying dist.
      name: "mesa-keep-dist",
      closeBundle() {
        writeFileSync(fileURLToPath(new URL("./dist/.gitkeep", import.meta.url)), "");
      },
    },
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      // `pnpm run dev` talks to a local `mesa-server`; the built app is embedded and served by it.
      "/api": { target: process.env.MESA_API ?? "http://localhost:8080", changeOrigin: true },
    },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
