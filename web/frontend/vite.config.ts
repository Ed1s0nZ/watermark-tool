import { defineConfig } from "vite";
export default defineConfig({
  build: {
    outDir: "../dist",
    emptyOutDir: true,
    rollupOptions: {
      // Ant Design's RSC markers are irrelevant in this client-only application.
      onwarn(warning, warn) {
        if (
          warning.code === "MODULE_LEVEL_DIRECTIVE" &&
          warning.message.includes("use client")
        )
          return;
        warn(warning);
      },
      output: {
        manualChunks(id) {
          if (id.includes("node_modules")) {
            if (
              id.includes("/antd/") ||
              id.includes("/@ant-design/") ||
              id.includes("/rc-") ||
              id.includes("/@rc-component/")
            )
              return "antd";
            return "vendor";
          }
        },
      },
    },
  },
  server: { proxy: { "/api": "http://127.0.0.1:8080" } },
});
