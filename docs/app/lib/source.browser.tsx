import type { ComponentType, ReactElement } from "react";
import browserCollections from "fumadocs-mdx:collections/browser";

type BrowserDocsCollection = {
  createClientLoader: (options: {
    id: string;
    component: (loaded: { default: ComponentType }) => ReactElement;
  }) => {
    useContent: (path: string) => ReactElement;
  };
};

const docs = (browserCollections as any)?.docs;

export const docsContent = docs.createClientLoader({
  id: "glowbom-docs",
  component: (loaded: { default: ComponentType }) => {
    const Content = loaded.default;
    return <Content />;
  },
});
