import type { Route } from "./+types/home";
import { DocsRouteView } from "../components/docs-route-view";
import { buildDocMeta } from "../lib/meta";
import { loadDocPage } from "../lib/source.server";

export async function loader() {
  return loadDocPage();
}

export function meta({ data }: Route.MetaArgs) {
  return buildDocMeta(data);
}

export default function Home({ loaderData }: Route.ComponentProps) {
  return <DocsRouteView loaderData={loaderData} />;
}
