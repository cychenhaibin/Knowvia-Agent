export type IncrementalSyncSummary = {
  kind:
    | 'incremental_sync'
    | 'incremental_sync_rate_limited'
    | 'incremental_sync_empty'
    | 'full_sync'
    | 'full_sync_empty';
  documents?: number;
  chunks?: number;
  added?: number;
  updated?: number;
  deleted?: number;
  unchanged?: number;
  deferred?: number;
  retryAfterSec?: number;
  addedTitles?: string[];
  updatedTitles?: string[];
  deletedTitles?: string[];
  deferredTitles?: string[];
  syncedTitles?: string[];
};

export type SyncErrorSummary = {
  kind: 'sync_error';
  provider: 'yuque' | 'feishu';
  code: string;
  message: string;
  logId?: string;
};

export type KnowledgeSyncSummary = IncrementalSyncSummary | SyncErrorSummary;

function isIncrementalKind(value: unknown): value is IncrementalSyncSummary['kind'] {
  return (
    value === 'incremental_sync' ||
    value === 'incremental_sync_rate_limited' ||
    value === 'incremental_sync_empty' ||
    value === 'full_sync' ||
    value === 'full_sync_empty'
  );
}

function isSyncErrorSummary(value: Record<string, unknown>): value is SyncErrorSummary {
  return (
    value.kind === 'sync_error' &&
    (value.provider === 'yuque' || value.provider === 'feishu') &&
    typeof value.code === 'string' &&
    typeof value.message === 'string'
  );
}

export function parseKnowledgeSyncSummary(summary: string): KnowledgeSyncSummary | null {
  const trimmed = summary.trim();
  if (!trimmed.startsWith('{')) {
    return null;
  }

  try {
    const parsed = JSON.parse(trimmed) as Record<string, unknown>;
    if (isIncrementalKind(parsed.kind)) {
      return parsed as IncrementalSyncSummary;
    }
    if (isSyncErrorSummary(parsed)) {
      return parsed;
    }
    return null;
  } catch {
    return null;
  }
}
