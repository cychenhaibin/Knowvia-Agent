import type {RunSource} from '@/types/api';

export type GroundedSourceRef = {
  key: string;
  title: string;
  provider: string;
  chunkId?: string;
  url?: string;
};

export function normalizeMarkdownLines(markdown: string) {
  return markdown
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

export function sourceIdentity(source: Pick<RunSource, 'chunkId' | 'title' | 'url' | 'snippet'>) {
  return source.chunkId || [source.title, source.url ?? '', source.snippet].join('|');
}

export function parseGroundedSourceRefs(markdown: string): GroundedSourceRef[] {
  const refs: GroundedSourceRef[] = [];
  const seen = new Set<string>();
  let current: Partial<GroundedSourceRef> | null = null;

  const pushCurrent = () => {
    if (!current?.title) {
      return;
    }
    const key = current.chunkId || `${current.title}|${current.url ?? ''}`;
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    refs.push({
      key,
      title: current.title,
      provider: current.provider || 'unknown',
      chunkId: current.chunkId,
      url: current.url,
    });
  };

  for (const rawLine of markdown.split('\n')) {
    const line = rawLine.trim();
    const sourceMatch = line.match(/^\d+\.\s+(.+)$/);
    if (sourceMatch) {
      pushCurrent();
      current = {title: sourceMatch[1], provider: 'unknown'};
      continue;
    }
    if (!current) {
      continue;
    }
    const providerMatch = line.match(/^- Provider:\s+`(.+)`$/);
    if (providerMatch) {
      current.provider = providerMatch[1];
      continue;
    }
    const chunkMatch = line.match(/^- Chunk:\s+`(.+)`$/);
    if (chunkMatch) {
      current.chunkId = chunkMatch[1];
      continue;
    }
    const urlMatch = line.match(/^- URL:\s+(.+)$/);
    if (urlMatch) {
      current.url = urlMatch[1];
    }
  }

  pushCurrent();
  return refs;
}
