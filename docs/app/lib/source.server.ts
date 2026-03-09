import { loader } from "fumadocs-core/source";
import { docs } from "fumadocs-mdx:collections/server";
import type { LoadedDocPage } from "./docs";

const baseUrl = process.env.DOCS_BASE_PATH ?? (process.env.NODE_ENV === "production" ? "/docs" : "/");

export const source = loader({
  baseUrl,
  source: docs.toFumadocsSource(),
});

const treePromise = source.serializePageTree(source.pageTree);

export async function loadDocPage(slugs?: string[]): Promise<LoadedDocPage> {
  const page = source.getPage(slugs);

  if (!page) {
    throw new Response("Not Found", { status: 404 });
  }

  return {
    tree: await treePromise,
    page: {
      path: page.path,
      title: page.data.title,
      description: page.data.description,
      toc: page.data.toc,
    },
  };
}
