import MarkdownIt from 'markdown-it';

const inlineMarkdown = new MarkdownIt({
  html: false,
  linkify: false,
  typographer: false,
});

inlineMarkdown.validateLink = (url: string) => {
  const lower = url.trim().toLowerCase();
  return lower.startsWith('http://') || lower.startsWith('https://') || lower.startsWith('mailto:');
};

const defaultLinkOpen =
  inlineMarkdown.renderer.rules.link_open ||
  ((tokens, idx, options, env, self) => self.renderToken(tokens, idx, options));

inlineMarkdown.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const token = tokens[idx];
  if (!token) {
    return defaultLinkOpen(tokens, idx, options, env, self);
  }
  token.attrSet('target', '_blank');
  token.attrSet('rel', 'noreferrer noopener');
  return defaultLinkOpen(tokens, idx, options, env, self);
};

export function lineLooksLikeMarkdown(line: string): boolean {
  const trimmed = line.trim();
  if (!trimmed) {
    return false;
  }
  if (trimmed.startsWith('#') || trimmed.startsWith('- ') || trimmed.startsWith('* ') || trimmed.startsWith('>')) {
    return true;
  }
  return trimmed.includes('**') || trimmed.includes('__') || trimmed.includes('`') || trimmed.includes('[');
}

export function parseConsoleHeading(line: string): { level: number; text: string } | null {
  const trimmed = line.trim();
  if (!trimmed.startsWith('#')) {
    return null;
  }

  let level = 0;
  for (const character of trimmed) {
    if (character === '#') {
      level += 1;
      continue;
    }
    break;
  }

  if (level <= 0 || level > 6) {
    return null;
  }

  const text = trimmed.slice(level).trim();
  if (!text) {
    return null;
  }

  return { level, text };
}

export function parseConsoleMarkdown(line: string): string | null {
  try {
    return inlineMarkdown.renderInline(line);
  } catch {
    return null;
  }
}
