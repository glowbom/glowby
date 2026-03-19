import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import mdx from "fumadocs-mdx/vite";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";
import * as sourceConfig from "./source.config";

const basePath = process.env.DOCS_BASE_PATH ?? (process.env.NODE_ENV === "production" ? "/docs" : "");

export default defineConfig(async () => ({
  base: basePath ? `${basePath}/` : "/",
  plugins: [tailwindcss(), await mdx(sourceConfig), reactRouter(), tsconfigPaths()],
}));
