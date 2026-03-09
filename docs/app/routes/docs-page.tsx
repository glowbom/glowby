import type { Route } from "./+types/docs-page";
import { DocsRouteView } from "../components/docs-route-view";
import { buildDocMeta } from "../lib/meta";
import { loadDocPage } from "../lib/source.server";

function toSlugs(value?: string): string[] | undefined {
  if (!value) {
    return undefined;
  }

  const slugs = value.split("/").filter(Boolean);
  return slugs.length > 0 ? slugs : undefined;
}

export async function loader({ params }: Route.LoaderArgs) {
  return loadDocPage(toSlugs(params["*"]));
}

export function meta({ data }: Route.MetaArgs) {
  return buildDocMeta(data);
}

export default function DocsPageRoute({ loaderData }: Route.ComponentProps) {
  return <DocsRouteView loaderData={loaderData} />;
}
