import {Ionicons} from '@expo/vector-icons';
import {useRouter} from 'expo-router';
import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {ActivityIndicator, Pressable, Text, View} from 'react-native';
import {useState} from 'react';

import {BottomSheet} from '@/components/BottomSheet';
import {ConfirmModal} from '@/components/ConfirmModal';
import {Screen} from '@/components/Screen';
import {TextField} from '@/components/TextField';
import {useI18n} from '@/i18n/useI18n';
import {api} from '@/lib/api';
import {ConnectorSheet} from '@/modules/knowledge/components/ConnectorSheet';
import {KnowledgeProviderIcon} from '@/modules/knowledge/components/KnowledgeProviderIcon';
import {parseKnowledgeSyncSummary} from '@/modules/knowledge/syncSummary';
import {useAuthStore} from '@/store/auth';
import {useAppTheme} from '@/theme/useAppTheme';
import type {KnowledgeConnection, KnowledgeSyncJob} from '@/types/api';

type ConnectionScope = 'team' | 'personal';
type KnowledgeProvider = 'yuque' | 'feishu';
type FeishuEntryType = 'docx' | 'wiki_node' | 'wiki_space';

function isActiveSyncJob(job?: KnowledgeSyncJob) {
  return job?.status === 'queued' || job?.status === 'running';
}

function formatKnowledgeSyncMessage(
  message: string,
  t: (key: any, params?: any) => string,
) {
  const trimmed = message.trim();
  if (!trimmed) {
    return trimmed;
  }

  const lower = trimmed.toLowerCase();
  const syncSummary = parseKnowledgeSyncSummary(trimmed);
  if (syncSummary?.kind === 'sync_error') {
    return syncSummary.logId ? `${syncSummary.message} (log_id: ${syncSummary.logId})` : syncSummary.message;
  }

  if (syncSummary?.kind === 'incremental_sync_empty') {
    return t('knowledge.syncResult.empty', {
      deleted: String(syncSummary.deleted ?? 0),
    });
  }

  if (syncSummary?.kind === 'incremental_sync') {
    return t('knowledge.syncResult.incremental', {
      documents: String(syncSummary.documents ?? 0),
      chunks: String(syncSummary.chunks ?? 0),
      added: String(syncSummary.added ?? 0),
      updated: String(syncSummary.updated ?? 0),
      deleted: String(syncSummary.deleted ?? 0),
    });
  }

  if (syncSummary?.kind === 'incremental_sync_rate_limited') {
    return t('knowledge.syncResult.rateLimited', {
      documents: String(syncSummary.documents ?? 0),
      chunks: String(syncSummary.chunks ?? 0),
      added: String(syncSummary.added ?? 0),
      updated: String(syncSummary.updated ?? 0),
      deleted: String(syncSummary.deleted ?? 0),
      deferred: String(syncSummary.deferred ?? 0),
    });
  }

  if (syncSummary?.kind === 'full_sync_empty') {
    return t('knowledge.syncResult.fullEmpty');
  }

  if (syncSummary?.kind === 'full_sync') {
    return t('knowledge.syncResult.full', {
      documents: String(syncSummary.documents ?? 0),
      chunks: String(syncSummary.chunks ?? 0),
    });
  }

  if (
    lower.includes('yuque api returned 429') ||
    lower.includes('too many requests')
  ) {
    const limitMatch = trimmed.match(/limit=([^\s)]+)/i);
    const remainingMatch = trimmed.match(/remaining=([^\s)]+)/i);
    const retryAfterMatch = trimmed.match(/retry_after=([^\s)]+)/i);
    const extras = [
      limitMatch ? `Limit ${limitMatch[1]}` : null,
      remainingMatch ? `Remaining ${remainingMatch[1]}` : null,
      retryAfterMatch ? `Retry-After ${retryAfterMatch[1]}` : null,
    ].filter(Boolean);
    if (extras.length > 0) {
      return `${t('knowledge.syncError.rateLimited')} (${extras.join(' · ')})`;
    }
    return t('knowledge.syncError.rateLimited');
  }

  if (
    lower.includes('context deadline exceeded') ||
    lower.includes('client.timeout exceeded while awaiting headers') ||
    lower.includes('read timed out')
  ) {
    if (lower.includes('open.feishu.cn') || lower.includes('feishu request failed')) {
      return t('knowledge.syncError.feishuTimeout');
    }
    return t('knowledge.syncError.timeout');
  }

  if (
    lower.includes('python backend unavailable') ||
    lower.includes('python knowledge sync unavailable')
  ) {
    return t('knowledge.syncError.backendUnavailable');
  }

  if (lower.startsWith('python proxy request failed:')) {
    return trimmed.replace(/^python proxy request failed:\s*/i, '');
  }

  if (lower.startsWith('yuque request failed:')) {
    return t('knowledge.syncError.network');
  }

  if (lower.startsWith('feishu request failed:')) {
    return t('knowledge.syncError.feishuNetwork');
  }

  return trimmed;
}

function ConnectionCard({
  accessToken,
  connection,
}: {
  accessToken: string;
  connection: KnowledgeConnection;
}) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const {colors} = useAppTheme();
  const {t} = useI18n();

  const syncJobsQuery = useQuery({
    queryKey: ['knowledge-sync-jobs', connection.id],
    queryFn: () => api.listSyncJobs(accessToken, connection.id),
    refetchInterval: (query) => {
      const items = (query.state.data as {items: KnowledgeSyncJob[]} | undefined)?.items ?? [];
      return isActiveSyncJob(items[0]) ? 2000 : false;
    },
  });

  const syncMutation = useMutation({
    mutationFn: () => api.syncConnection(accessToken, connection.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({queryKey: ['knowledge-connections']});
      await queryClient.invalidateQueries({queryKey: ['knowledge-sync-jobs', connection.id]});
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteConnection(accessToken, connection.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({queryKey: ['knowledge-connections']});
      queryClient.removeQueries({queryKey: ['knowledge-sync-jobs', connection.id]});
      queryClient.removeQueries({queryKey: ['knowledge-connection', connection.id]});
    },
  });

  const latestJob = syncJobsQuery.data?.items[0];
  const activeSync = isActiveSyncJob(latestJob);
  const actionsDisabled = activeSync || syncMutation.isPending || deleteMutation.isPending;
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const providerTitle =
    connection.provider === 'yuque'
      ? t('knowledge.provider.yuque.title')
      : t('knowledge.provider.feishu.title');
  const connectionHint =
    connection.provider === 'yuque'
      ? `${connection.yuque?.groupLogin ?? ''}${connection.yuque?.namespace ? ` / ${connection.yuque.namespace}` : ''}`
      : `${providerTitle} · ${connection.feishu?.entryType ?? 'docx'} · ${connection.feishu?.entryToken ?? ''}`;
  const providerAccent =
    connection.provider === 'yuque' ? '#16A34A' : '#246BFD';
  const yuqueSyncNotice = connection.provider === 'yuque' ? t('knowledge.yuqueSyncNotice') : null;

  let syncStatusText = connection.lastSyncedAt
    ? t('knowledge.lastSynced', {
        time: new Date(connection.lastSyncedAt).toLocaleString(),
      })
    : t('knowledge.neverSynced');
  let syncStatusColor: string = colors.textTertiary;
  const mutationError =
    deleteMutation.error instanceof Error
      ? deleteMutation.error.message
      : syncMutation.error instanceof Error
        ? syncMutation.error.message
        : null;

  if (latestJob?.status === 'queued' || latestJob?.status === 'running' || latestJob?.status === 'failed') {
    syncStatusText = latestJob.summary;
  } else if (latestJob?.status === 'completed' && latestJob.summary) {
    syncStatusText = connection.lastSyncedAt
      ? `${t('knowledge.lastSynced', {
          time: new Date(connection.lastSyncedAt).toLocaleString(),
        })} · ${latestJob.summary}`
      : latestJob.summary;
  }
  if (latestJob?.status === 'running' || latestJob?.status === 'queued') {
    syncStatusColor = colors.brand;
  } else if (latestJob?.status === 'failed' || syncMutation.error) {
    syncStatusColor = '#dc2626';
  } else if (latestJob?.status === 'completed') {
    syncStatusColor = colors.textSecondary;
  }

  if (mutationError) {
    syncStatusColor = '#dc2626';
  }

  const displaySyncStatus = formatKnowledgeSyncMessage(mutationError ?? syncStatusText, t);

  const canOpenDetails =
    !actionsDisabled &&
    !!connection.lastSyncedAt &&
    (!latestJob || latestJob.status === 'completed');

  return (
    <>
      <View className="gap-3 rounded-[12px] p-5" style={{backgroundColor: colors.surface}}>
        <View className="flex-row items-start gap-3">
          <View
            className="h-12 w-12 items-center justify-center rounded-[16px]"
            style={{backgroundColor: colors.surfaceMuted}}>
            <KnowledgeProviderIcon provider={connection.provider} size={32} />
          </View>
          <View className="flex-1 gap-2">
            <Text className="text-lg font-semibold" style={{color: colors.textPrimary}}>
              {connection.name}
            </Text>
            <Text className="text-sm" style={{color: colors.textSecondary}}>
              {connectionHint}
            </Text>
          </View>

          <View className="flex-row items-center gap-2">
            <Pressable
              className="h-10 w-10 items-center justify-center rounded-[12px]"
              style={{
                backgroundColor: actionsDisabled ? colors.surfaceMuted : colors.brandSoft,
                borderWidth: 1,
                borderColor: actionsDisabled ? colors.divider : '#f24f2d33',
                opacity: actionsDisabled ? 0.75 : 1,
              }}
              disabled={actionsDisabled}
              onPress={() => syncMutation.mutate()}>
              {activeSync || syncMutation.isPending ? (
                <ActivityIndicator size="small" color={colors.textTertiary} />
              ) : (
                <Ionicons
                  name="refresh-outline"
                  size={18}
                  color={actionsDisabled ? colors.textTertiary : colors.brand}
                />
              )}
            </Pressable>
            <Pressable
              className="h-10 w-10 items-center justify-center rounded-[12px]"
              style={{
                backgroundColor: actionsDisabled ? colors.surfaceMuted : colors.brandSoft,
                borderWidth: 1,
                borderColor: actionsDisabled ? colors.divider : '#f24f2d33',
                opacity: actionsDisabled ? 0.75 : 1,
              }}
              disabled={actionsDisabled}
              onPress={() => setShowDeleteConfirm(true)}>
              {deleteMutation.isPending ? (
                <ActivityIndicator size="small" color={colors.textTertiary} />
              ) : (
                <Ionicons
                  name="trash-outline"
                  size={18}
                  color={actionsDisabled ? colors.textTertiary : colors.brand}
                />
              )}
            </Pressable>
          </View>
        </View>

        {canOpenDetails ? (
          <View className="gap-2">
            <Pressable
              className="gap-2 rounded-[12px] p-3"
              style={{backgroundColor: colors.surfaceMuted}}
              onPress={() => router.push(`/knowledge/${connection.id}`)}>
              <Text className="text-sm leading-6" style={{color: syncStatusColor}}>
                {displaySyncStatus}
              </Text>
              <View className="flex-row items-center justify-between">
                <Text className="text-xs font-medium" style={{color: colors.textTertiary}}>
                  {t('knowledge.viewSynced')}
                </Text>
                <Ionicons name="chevron-forward" size={16} color={colors.textTertiary} />
              </View>
            </Pressable>
            {yuqueSyncNotice ? (
              <Text className="text-xs leading-5" style={{color: '#dc2626'}}>
                {yuqueSyncNotice}
              </Text>
            ) : null}
          </View>
        ) : (
          <View className="gap-2">
            <Text className="text-sm leading-6" style={{color: syncStatusColor}}>
              {displaySyncStatus}
            </Text>
            {yuqueSyncNotice ? (
              <Text className="text-xs leading-5" style={{color: '#dc2626'}}>
                {yuqueSyncNotice}
              </Text>
            ) : null}
          </View>
        )}
      </View>

      <ConfirmModal
        visible={showDeleteConfirm}
        title={t('knowledge.deleteConnectionConfirmTitle')}
        body={t('knowledge.deleteConnectionConfirmBody', {name: connection.name})}
        cancelLabel={t('common.cancel')}
        confirmLabel={t('knowledge.deleteConnection')}
        onCancel={() => setShowDeleteConfirm(false)}
        onConfirm={() => {
          setShowDeleteConfirm(false);
          deleteMutation.mutate();
        }}
      />
    </>
  );
}

export default function KnowledgeScreen() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const accessToken = useAuthStore((state) => state.accessToken)!;
  const [showAddConnectionSheet, setShowAddConnectionSheet] = useState(false);
  const [showProviderSheet, setShowProviderSheet] = useState(false);
  const [provider, setProvider] = useState<KnowledgeProvider>('yuque');
  const [name, setName] = useState('');
  const [token, setToken] = useState('');
  const [groupLogin, setGroupLogin] = useState('');
  const [namespace, setNamespace] = useState('');
  const [feishuAppId, setFeishuAppId] = useState('');
  const [feishuAppSecret, setFeishuAppSecret] = useState('');
  const [feishuDocumentInput, setFeishuDocumentInput] = useState('');
  const [feishuEntryType, setFeishuEntryType] = useState<FeishuEntryType>('wiki_node');
  const [connectionScope, setConnectionScope] = useState<ConnectionScope>('team');
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const loginLabel =
    connectionScope === 'team' ? t('knowledge.groupLogin') : t('knowledge.personalLogin');
  const loginPlaceholder =
    connectionScope === 'team'
      ? t('knowledge.placeholder.groupLogin')
      : t('knowledge.placeholder.personalLogin');
  const namespacePlaceholder =
    connectionScope === 'team'
      ? t('knowledge.placeholder.namespace')
      : t('knowledge.placeholder.personalNamespace');
  const scopeHint =
    connectionScope === 'team'
      ? t('knowledge.scopeHint.team')
      : t('knowledge.scopeHint.personal');

  const resetConnectionForm = () => {
    setProvider('yuque');
    setName(t('knowledge.defaultName.yuque'));
    setToken('');
    setGroupLogin('');
    setNamespace('');
    setFeishuAppId('');
    setFeishuAppSecret('');
    setFeishuDocumentInput('');
    setFeishuEntryType('wiki_node');
    setConnectionScope('team');
  };

  const connectionsQuery = useQuery({
    queryKey: ['knowledge-connections'],
    queryFn: () => api.listConnections(accessToken),
  });

  const createMutation = useMutation({
    mutationFn: () =>
      provider === 'yuque'
        ? api.createConnection(accessToken, {
            provider: 'yuque',
            name,
            syncEnabled: true,
            config: {
              token,
              groupLogin,
              namespace,
            },
          })
        : api.createConnection(accessToken, {
            provider: 'feishu',
            name,
            syncEnabled: true,
            config: {
              appId: feishuAppId,
              appSecret: feishuAppSecret,
              entryType: feishuEntryType,
              entryToken: feishuDocumentInput,
            },
          }),
    onSuccess: async () => {
      resetConnectionForm();
      setShowAddConnectionSheet(false);
      await queryClient.invalidateQueries({queryKey: ['knowledge-connections']});
    },
  });

  const createError =
    createMutation.error instanceof Error
      ? createMutation.error.message
      : null;
  const submitLabel = createMutation.isPending
    ? t('knowledge.savingConnection')
    : t('knowledge.saveConnection');
  const connectorTitle =
    provider === 'yuque'
      ? t('knowledge.addConnection')
      : t('knowledge.connectorTitle.feishu');
  const connectorDescription =
    provider === 'yuque'
      ? t('knowledge.connectorDescription.yuque')
      : t('knowledge.connectorDescription.feishu');
  const feishuEntryHint =
    feishuEntryType === 'docx'
      ? t('knowledge.feishuEntryHint.docx')
      : feishuEntryType === 'wiki_node'
        ? t('knowledge.feishuEntryHint.wikiNode')
        : t('knowledge.feishuEntryHint.wikiSpace');
  const providerOptions = [
    {
      key: 'yuque' as const,
      title: t('knowledge.provider.yuque.title'),
      description: t('knowledge.provider.yuque.description'),
    },
    {
      key: 'feishu' as const,
      title: t('knowledge.provider.feishu.title'),
      description: t('knowledge.provider.feishu.description'),
    },
  ];

  const renderConnectorFields = () => {
    if (provider === 'yuque') {
      return (
        <>
          <TextField
            label={t('knowledge.connectionName')}
            value={name}
            onChangeText={setName}
            placeholder={t('knowledge.placeholder.connectionName')}
          />
          <View className="flex-row items-center justify-center gap-3">
            <View
              className="flex-row rounded-full p-1"
              style={{backgroundColor: colors.surfaceMuted}}>
              {(['team', 'personal'] as const).map((scope) => {
                const active = connectionScope === scope;
                return (
                  <Pressable
                    key={scope}
                    className="rounded-full px-6 py-2"
                    style={{
                      backgroundColor: active ? colors.brand : 'transparent',
                    }}
                    onPress={() => setConnectionScope(scope)}>
                    <Text
                      className={active ? 'font-semibold' : undefined}
                      style={{color: active ? colors.surface : colors.textPrimary}}>
                      {scope === 'team'
                        ? t('knowledge.scope.team')
                        : t('knowledge.scope.personal')}
                    </Text>
                  </Pressable>
                );
              })}
            </View>
          </View>
          <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
            {scopeHint}
          </Text>
          <TextField
            label={loginLabel}
            value={groupLogin}
            onChangeText={setGroupLogin}
            placeholder={loginPlaceholder}
          />
          <TextField
            label={t('knowledge.namespaceOptional')}
            value={namespace}
            onChangeText={setNamespace}
            placeholder={namespacePlaceholder}
          />
          <TextField
            label={t('knowledge.token')}
            value={token}
            onChangeText={setToken}
            placeholder={t('knowledge.placeholder.token')}
          />
        </>
      );
    }

    return (
      <>
        <TextField
          label={t('knowledge.connectionName')}
          value={name}
          onChangeText={setName}
          placeholder={t('knowledge.placeholder.feishuConnectionName')}
        />
        <TextField
          label={t('knowledge.feishuAppId')}
          value={feishuAppId}
          onChangeText={setFeishuAppId}
          placeholder={t('knowledge.placeholder.feishuAppId')}
        />
        <TextField
          label={t('knowledge.feishuAppSecret')}
          value={feishuAppSecret}
          onChangeText={setFeishuAppSecret}
          placeholder={t('knowledge.placeholder.feishuAppSecret')}
          secureTextEntry
        />
        <View className="flex-row items-center justify-center gap-3">
          <View
            className="flex-row rounded-full p-1"
            style={{backgroundColor: colors.surfaceMuted}}>
            {(['wiki_node', 'docx', 'wiki_space'] as const).map((entryType) => {
              const active = feishuEntryType === entryType;
              return (
                <Pressable
                  key={entryType}
                  className="rounded-full px-4 py-2"
                  style={{
                    backgroundColor: active ? colors.brand : 'transparent',
                  }}
                  onPress={() => setFeishuEntryType(entryType)}>
                  <Text
                    className={active ? 'font-semibold' : undefined}
                    style={{color: active ? colors.surface : colors.textPrimary}}>
                    {t(`knowledge.feishuEntryType.${entryType}` as any)}
                  </Text>
                </Pressable>
              );
            })}
          </View>
        </View>
        <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
          {feishuEntryHint}
        </Text>
        <TextField
          label={t('knowledge.feishuEntryToken')}
          value={feishuDocumentInput}
          onChangeText={setFeishuDocumentInput}
          placeholder={t(`knowledge.placeholder.feishuEntryToken.${feishuEntryType}` as any)}
        />
      </>
    );
  };

  return (
    <Screen scroll>
      <View className="gap-6">
        <View className="gap-3">
          <View className="flex-row items-center gap-3">
            <Pressable
              className="h-10 w-10 items-center justify-center rounded-full"
              style={{backgroundColor: colors.surface}}
              onPress={() => router.back()}>
              <Ionicons name="arrow-back" size={20} color={colors.textPrimary} />
            </Pressable>
          </View>
          <View className="flex-row items-center justify-between">
            <Text className="text-3xl font-bold" style={{color: colors.textPrimary}}>
              {t('knowledge.title')}
            </Text>
            <Pressable
              className="h-10 w-10 items-center justify-center rounded-full"
              style={{backgroundColor: colors.surface}}
              onPress={() => {
                createMutation.reset();
                setShowAddConnectionSheet(false);
                setShowProviderSheet(true);
              }}>
              <Ionicons name="add" size={24} color={colors.textPrimary} />
            </Pressable>
          </View>
          <Text className="text-base leading-7" style={{color: colors.textSecondary}}>
            {t('knowledge.description')}
          </Text>
        </View>

        <View className="gap-4">
          {connectionsQuery.data?.items.map((connection) => (
            <ConnectionCard
              key={connection.id}
              accessToken={accessToken}
              connection={connection}
            />
          ))}
          {connectionsQuery.error ? (
            <Text className="text-sm text-red-600">
              {connectionsQuery.error instanceof Error
                ? connectionsQuery.error.message
                : t('knowledge.description')}
            </Text>
          ) : null}
        </View>
      </View>

      <BottomSheet
        visible={showProviderSheet}
        onClose={() => setShowProviderSheet(false)}
        title={t('knowledge.sourcePickerTitle')}>
        <View>
          {providerOptions.map((item) => (
            <Pressable
              key={item.key}
              className="mb-3 flex-row items-start gap-3 rounded-[18px] p-4"
              style={{backgroundColor: colors.surfaceMuted}}
              onPress={() => {
                createMutation.reset();
                setProvider(item.key);
                if (item.key === 'yuque') {
                  setName(t('knowledge.defaultName.yuque'));
                } else {
                  setName(t('knowledge.defaultName.feishu'));
                }
                setShowProviderSheet(false);
                requestAnimationFrame(() => {
                  setShowAddConnectionSheet(true);
                });
              }}>
              <View
                className="h-10 w-10 items-center justify-center rounded-full"
                style={{backgroundColor: colors.surface}}>
                <KnowledgeProviderIcon provider={item.key} size={22} />
              </View>
              <View className="flex-1 gap-1">
                <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
                  {item.title}
                </Text>
                <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
                  {item.description}
                </Text>
              </View>
              <Ionicons name="chevron-forward" size={18} color={colors.textMuted} />
            </Pressable>
          ))}
        </View>
      </BottomSheet>

      <ConnectorSheet
        visible={showAddConnectionSheet && !showProviderSheet}
        onClose={() => {
          setShowAddConnectionSheet(false);
          createMutation.reset();
        }}
        title={connectorTitle}
        description={connectorDescription}
        error={createError}
        submitLabel={submitLabel}
        submitting={createMutation.isPending}
        onSubmit={() => createMutation.mutate()}>
        {renderConnectorFields()}
      </ConnectorSheet>
    </Screen>
  );
}
