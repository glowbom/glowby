import type { Config } from "@react-router/dev/config";

const basename = process.env.DOCS_BASE_PATH ?? (process.env.NODE_ENV === "production" ? "/docs" : "/");

export default {
  ssr: true,
  basename,
  routeDiscovery: {
    mode: "initial",
  },
} satisfies Config;
