import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import mdx from "fumadocs-mdx/vite";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";
import * as sourceConfig from "./source.config";

export default defineConfig(async () => ({
  plugins: [tailwindcss(), await mdx(sourceConfig), reactRouter(), tsconfigPaths()],
}));
