import { Suspense } from "react";
import { deserializePageTree } from "fumadocs-core/source/client";
import { DocsLayout } from "fumadocs-ui/layouts/docs";
import { DocsBody, DocsDescription, DocsPage, DocsTitle } from "fumadocs-ui/page";
import type { LoadedDocPage } from "../lib/docs";
import { BrandTitle } from "./brand-title";
import {
  DocsSearchDialog,
  DocsSearchLargeTrigger,
  DocsSearchProvider,
  DocsSearchSmallTrigger,
} from "./docs-search";
import { NavActions } from "./nav-actions";
import { docsContent } from "../lib/source.browser";

export function DocsRouteView({ loaderData }: { loaderData: LoadedDocPage }) {
  const tree = deserializePageTree(loaderData.tree);

  console.log("Search records count =>", loaderData.searchRecords?.length);

  return (
    <DocsSearchProvider records={loaderData.searchRecords}>
      <DocsLayout
        tree={tree}
        nav={{ title: <BrandTitle />, url: "/", children: <NavActions /> }}
        searchToggle={{
          enabled: true,
          components: {
            lg: <DocsSearchLargeTrigger />,
            sm: <DocsSearchSmallTrigger />,
          },
        }}
        themeSwitch={{ enabled: true, mode: "light-dark-system" }}
        sidebar={{ defaultOpenLevel: 1 }}
      >
        <DocsPage toc={loaderData.page.toc as never}>
          <DocsTitle>{loaderData.page.title}</DocsTitle>
          <DocsDescription>{loaderData.page.description}</DocsDescription>
          <DocsBody>
            <Suspense
              fallback={<p className="text-sm text-fd-muted-foreground">Loading page content...</p>}
            >
              {docsContent.useContent(loaderData.page.path)}
            </Suspense>
          </DocsBody>
        </DocsPage>
      </DocsLayout>
      <DocsSearchDialog />
    </DocsSearchProvider>
  );
}
