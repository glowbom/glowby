import type { SerializedPageTree } from "fumadocs-core/source/client";

export interface LoadedDocPage {
  tree: SerializedPageTree;
  page: {
    path: string;
    title: string;
    description?: string;
    toc: unknown[];
  };
}
