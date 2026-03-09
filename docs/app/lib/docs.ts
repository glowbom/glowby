import type { SerializedPageTree } from "fumadocs-core/source/client";

export interface SearchRecord {
  id: string;
  url: string;
  type: "page" | "heading" | "text";
  content: string;
  breadcrumbs: string[];
  text: string;
}

export interface LoadedDocPage {
  tree: SerializedPageTree;
  page: {
    path: string;
    title: string;
    description?: string;
    toc: unknown[];
  };
  searchRecords: SearchRecord[];
}
