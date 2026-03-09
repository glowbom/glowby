import type { LoadedDocPage } from "./docs";

export function buildDocMeta(data?: LoadedDocPage) {
  if (!data) {
    return [{ title: "Not Found | Glowbom Docs" }];
  }

  const title = data.page.title === "Glowbom Docs" ? data.page.title : `${data.page.title} | Glowbom Docs`;
  const meta: Array<{ title: string } | { name: string; content: string }> = [{ title }];

  if (data.page.description) {
    meta.push({
      name: "description",
      content: data.page.description,
    });
  }

  return meta;
}
