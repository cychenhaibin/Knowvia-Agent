import {Ionicons} from '@expo/vector-icons';
import {useQuery} from '@tanstack/react-query';
import {useLocalSearchParams, useRouter} from 'expo-router';
import {Linking, Pressable, Text, View} from 'react-native';

import {Screen} from '@/components/Screen';
import {useI18n} from '@/i18n/useI18n';
import {api} from '@/lib/api';
import {parseKnowledgeSyncSummary} from '@/modules/knowledge/syncSummary';
import {useAuthStore} from '@/store/auth';
import {useAppTheme} from '@/theme/useAppTheme';

function StatCard({
  label,
  value,
}: {
  label: string;
  value: string;
}) {
  const {colors} = useAppTheme();

  return (
    <View className="flex-1 rounded-[20px] p-4" style={{backgroundColor: colors.surface}}>
      <Text className="text-xs uppercase tracking-[1.5px]" style={{color: colors.textTertiary}}>
        {label}
      </Text>
      <Text className="mt-2 text-2xl font-semibold" style={{color: colors.textPrimary}}>
        {value}
      </Text>
    </View>
  );
}

export default function KnowledgeDetailScreen() {
  const router = useRouter();
  const {id} = useLocalSearchParams<{id: string}>();
  const accessToken = useAuthStore((state) => state.accessToken)!;
  const {colors} = useAppTheme();
  const {t} = useI18n();

  const detailQuery = useQuery({
    queryKey: ['knowledge-connection', id],
    queryFn: () => api.getConnectionDocuments(accessToken, id!),
    enabled: !!id,
  });
  const syncJobsQuery = useQuery({
    queryKey: ['knowledge-sync-jobs', id],
    queryFn: () => api.listSyncJobs(accessToken, id!),
    enabled: !!id,
  });

  if (!detailQuery.data) {
    return (
      <Screen>
        <View className="flex-row items-center gap-3">
          <Pressable
            className="h-10 w-10 items-center justify-center rounded-full"
            style={{backgroundColor: colors.surface}}
            onPress={() => router.back()}>
            <Ionicons name="arrow-back" size={20} color={colors.textPrimary} />
          </Pressable>
          <Text className="text-lg font-semibold" style={{color: colors.textPrimary}}>
            {t('knowledge.detailTitle')}
          </Text>
        </View>
        <View className="flex-1 items-center justify-center">
          <Text
            className="text-base"
            style={{color: detailQuery.error ? '#dc2626' : colors.textSecondary}}>
            {detailQuery.error instanceof Error
              ? detailQuery.error.message
              : t('knowledge.detailLoading')}
          </Text>
        </View>
      </Screen>
    );
  }

  const {connection, repos, documents, repoCount, documentCount, chunkCount} = detailQuery.data;
  const latestSyncJob = syncJobsQuery.data?.items[0];
  const latestSyncSummary = latestSyncJob ? parseKnowledgeSyncSummary(latestSyncJob.summary) : null;
  const connectionPath =
    connection.provider === 'yuque'
      ? connection.yuque?.namespace
        ? `${connection.yuque.groupLogin} / ${connection.yuque.namespace}`
        : connection.yuque?.groupLogin ?? ''
      : `${connection.feishu?.entryType ?? 'docx'} / ${connection.feishu?.entryToken ?? ''}`;

  return (
    <Screen scroll>
      <View className="gap-6 pb-12">
        <View className="flex-row items-center gap-3">
          <Pressable
            className="h-10 w-10 items-center justify-center rounded-full"
            style={{backgroundColor: colors.surface}}
            onPress={() => router.back()}>
            <Ionicons name="arrow-back" size={20} color={colors.textPrimary} />
          </Pressable>
          <View className="flex-1">
            <Text className="text-lg font-semibold" style={{color: colors.textPrimary}}>
              {t('knowledge.detailTitle')}
            </Text>
            <Text className="text-sm" style={{color: colors.textSecondary}}>
              {connection.name}
            </Text>
          </View>
        </View>

        <View className="gap-3 rounded-[28px] p-5" style={{backgroundColor: colors.surface}}>
          <Text className="text-2xl font-bold" style={{color: colors.textPrimary}}>
            {connection.name}
          </Text>
          <Text className="text-base leading-7" style={{color: colors.textSecondary}}>
            {connectionPath}
          </Text>
          <Text className="text-sm leading-6" style={{color: colors.textTertiary}}>
            {connection.lastSyncedAt
              ? t('knowledge.lastSynced', {
                  time: new Date(connection.lastSyncedAt).toLocaleString(),
                })
              : t('knowledge.neverSynced')}
          </Text>
        </View>

        <View className="flex-row gap-3">
          <StatCard label={t('knowledge.repos')} value={String(repoCount)} />
          <StatCard label={t('knowledge.documents')} value={String(documentCount)} />
          <StatCard label={t('knowledge.chunks')} value={String(chunkCount)} />
        </View>

        {latestSyncSummary ? (
          <View className="gap-3">
            <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
              {t('knowledge.latestSyncTitle')}
            </Text>
            <View className="gap-3 rounded-[20px] p-4" style={{backgroundColor: colors.surface}}>
              <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
                {latestSyncSummary.kind === 'sync_error'
                  ? latestSyncSummary.logId
                    ? `${latestSyncSummary.message} (log_id: ${latestSyncSummary.logId})`
                    : latestSyncSummary.message
                  : latestSyncSummary.kind === 'incremental_sync_empty'
                  ? t('knowledge.syncResult.empty', {
                      deleted: String(latestSyncSummary.deleted ?? 0),
                    })
                  : latestSyncSummary.kind === 'incremental_sync_rate_limited'
                    ? t('knowledge.syncResult.rateLimited', {
                        documents: String(latestSyncSummary.documents ?? 0),
                        chunks: String(latestSyncSummary.chunks ?? 0),
                        added: String(latestSyncSummary.added ?? 0),
                        updated: String(latestSyncSummary.updated ?? 0),
                        deleted: String(latestSyncSummary.deleted ?? 0),
                        deferred: String(latestSyncSummary.deferred ?? 0),
                      })
                  : latestSyncSummary.kind === 'full_sync_empty'
                    ? t('knowledge.syncResult.fullEmpty')
                    : latestSyncSummary.kind === 'full_sync'
                      ? t('knowledge.syncResult.full', {
                          documents: String(latestSyncSummary.documents ?? 0),
                          chunks: String(latestSyncSummary.chunks ?? 0),
                        })
                  : t('knowledge.syncResult.incremental', {
                      documents: String(latestSyncSummary.documents ?? 0),
                      chunks: String(latestSyncSummary.chunks ?? 0),
                      added: String(latestSyncSummary.added ?? 0),
                      updated: String(latestSyncSummary.updated ?? 0),
                      deleted: String(latestSyncSummary.deleted ?? 0),
                    })}
              </Text>
              {latestSyncSummary.kind !== 'sync_error' ? (
                <>
                  <SyncChangeSection
                    title={t('knowledge.syncResult.addedTitle')}
                    items={latestSyncSummary.addedTitles}
                  />
                  <SyncChangeSection
                    title={t('knowledge.syncResult.updatedTitle')}
                    items={latestSyncSummary.updatedTitles}
                  />
                  <SyncChangeSection
                    title={t('knowledge.syncResult.deletedTitle')}
                    items={latestSyncSummary.deletedTitles}
                  />
                  <SyncChangeSection
                    title={t('knowledge.syncResult.deferredTitle')}
                    items={latestSyncSummary.deferredTitles}
                  />
                  <SyncChangeSection
                    title={t('knowledge.syncResult.syncedTitle')}
                    items={latestSyncSummary.syncedTitles}
                  />
                </>
              ) : null}
            </View>
          </View>
        ) : null}

        <View className="gap-3">
          <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
            {t('knowledge.repos')}
          </Text>
          {repos.length ? (
            repos.map((repo) => (
              <View
                key={repo.name}
                className="rounded-[20px] p-4"
                style={{backgroundColor: colors.surface}}>
                <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
                  {repo.name}
                </Text>
                <Text className="mt-1 text-sm" style={{color: colors.textSecondary}}>
                  {t('knowledge.documentCountLabel', {count: repo.documentCount})}
                </Text>
              </View>
            ))
          ) : (
            <View className="rounded-[20px] p-4" style={{backgroundColor: colors.surface}}>
              <Text className="text-sm" style={{color: colors.textSecondary}}>
                {t('knowledge.emptyDocuments')}
              </Text>
            </View>
          )}
        </View>

        <View className="gap-3">
          <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
            {t('knowledge.documents')}
          </Text>
          {documents.length ? (
            documents.map((doc) => {
              const content = (
                <View
                  className="rounded-[20px] p-4"
                  style={{backgroundColor: colors.surface}}>
                  <View className="flex-row items-start justify-between gap-3">
                    <View className="flex-1 gap-2">
                      <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
                        {doc.title}
                      </Text>
                      <Text className="text-sm" style={{color: colors.textSecondary}}>
                        {doc.repo}
                      </Text>
                      <Text className="text-xs" style={{color: colors.textTertiary}}>
                        {new Date(doc.updatedAt).toLocaleString()}
                      </Text>
                    </View>
                    {doc.sourceUrl ? (
                      <Ionicons name="open-outline" size={18} color={colors.textTertiary} />
                    ) : null}
                  </View>
                </View>
              );

              if (!doc.sourceUrl) {
                return <View key={doc.id}>{content}</View>;
              }

              return (
                <Pressable
                  key={doc.id}
                  onPress={() => {
                    void Linking.openURL(doc.sourceUrl!);
                  }}>
                  {content}
                </Pressable>
              );
            })
          ) : (
            <View className="rounded-[20px] p-4" style={{backgroundColor: colors.surface}}>
              <Text className="text-sm" style={{color: colors.textSecondary}}>
                {t('knowledge.emptyDocuments')}
              </Text>
            </View>
          )}
        </View>
      </View>
    </Screen>
  );
}

function SyncChangeSection({
  title,
  items,
}: {
  title: string;
  items?: string[];
}) {
  const {colors} = useAppTheme();

  if (!items?.length) {
    return null;
  }

  return (
    <View className="gap-2">
      <Text className="text-sm font-semibold" style={{color: colors.textPrimary}}>
        {title}
      </Text>
      <View className="gap-1">
        {items.map((item) => (
          <Text key={item} className="text-sm leading-6" style={{color: colors.textSecondary}}>
            {'\u2022'} {item}
          </Text>
        ))}
      </View>
    </View>
  );
}
