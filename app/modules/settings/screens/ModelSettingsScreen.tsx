import {Ionicons} from '@expo/vector-icons';
import {useQuery} from '@tanstack/react-query';
import {useRouter} from 'expo-router';
import {useMemo, useState} from 'react';
import {
  ActivityIndicator,
  Modal,
  Pressable,
  ScrollView,
  Text,
  View,
} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {PrimaryButton} from '@/components/PrimaryButton';
import {ConfirmModal} from '@/components/ConfirmModal';
import {useTatos} from '@/components/Tatos';
import {TextField} from '@/components/TextField';
import {useI18n} from '@/i18n/useI18n';
import {api} from '@/lib/api';
import {queryClient} from '@/lib/query-client';
import {useAuthStore} from '@/store/auth';
import type {ChatModelPurpose, UserChatModel} from '@/types/api';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

type ModelDraft = {
  name: string;
  baseUrl: string;
  apiKey: string;
  modelName: string;
};

function normalizeModelConfigName(name: string) {
  return name.trim().toLocaleLowerCase();
}

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}

function modelSettingsErrorMessage(error: unknown, t: ReturnType<typeof useI18n>['t']) {
  const message = errorMessage(error, t('models.loadFailed'));
  if (message.trim().toLowerCase() === 'route not found') {
    return t('models.routeNotFoundHint');
  }
  return message;
}

function modelMutationErrorMessage(error: unknown, t: ReturnType<typeof useI18n>['t']) {
  const message = errorMessage(error, t('models.addFailed'));
  const normalized = message.trim().toLowerCase();

  if (
    normalized === 'model configuration already exists for this purpose' ||
    (normalized.includes('duplicate key value') &&
      normalized.includes('idx_user_chat_models_user_purpose_name'))
  ) {
    return t('models.duplicate');
  }

  return message;
}

function hasDuplicateModelName(models: UserChatModel[], name: string, excludedModelId?: string) {
  const normalizedName = normalizeModelConfigName(name);
  if (!normalizedName) {
    return false;
  }
  return models.some(
    (model) =>
      model.id !== excludedModelId && normalizeModelConfigName(model.name) === normalizedName,
  );
}

function StatusPill({
  label,
  tone = 'brand',
}: {
  label: string;
  tone?: 'brand' | 'muted';
}) {
  const {colors} = useAppTheme();
  const backgroundColor = tone === 'brand' ? colors.brandSoft : colors.surfaceMuted;
  const textColor = tone === 'brand' ? colors.brand : colors.textSecondary;

  return (
    <View className="rounded-full px-2.5 py-1" style={{backgroundColor}}>
      <Text style={{fontSize: fontSizes.xs, fontWeight: '600', color: textColor}}>{label}</Text>
    </View>
  );
}

function ModelRow({
  model,
  allowSelect,
  showSelectedState,
  busy,
  onEdit,
  onSelect,
  onDelete,
}: {
  model: UserChatModel;
  allowSelect: boolean;
  showSelectedState: boolean;
  busy: boolean;
  onEdit: () => void;
  onSelect?: () => void;
  onDelete: () => void;
}) {
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const deleteLocked = model.origin === 'default';
  const editLocked = model.origin === 'default';
  const unavailable = model.origin === 'default' && model.available === false;
  const deleteDisabled = busy || deleteLocked;
  const selectDisabled = busy || unavailable;
  const selectionIcon = model.isSelected ? 'radio-button-on' : 'radio-button-off-outline';

  return (
    <View
      className="flex-row items-center gap-3 rounded-[18px] px-4 py-3"
      style={{backgroundColor: colors.surfaceMuted}}>
      <View className="flex-1 gap-1.5">
        <View className="flex-row items-center gap-2">
          <Text
            numberOfLines={1}
            style={{
              fontSize: fontSizes.sm,
              color: unavailable ? colors.textMuted : colors.textPrimary,
            }}>
            {model.name}
          </Text>
          {deleteLocked ? <StatusPill label={t('models.default')} tone="muted" /> : null}
          {showSelectedState && model.isSelected ? <StatusPill label={t('models.selected')} /> : null}
        </View>
        <Text
          numberOfLines={1}
          style={{
            fontSize: fontSizes.xs,
            color: unavailable ? colors.textTertiary : colors.textSecondary,
          }}>
          {model.modelName}
        </Text>
        <Text
          numberOfLines={1}
          style={{fontSize: fontSizes.xs, color: colors.textTertiary}}>
          {model.baseUrl}
        </Text>
      </View>

      <View className="flex-row items-center gap-1">
        {allowSelect ? (
          <Pressable
            className="h-10 w-10 items-center justify-center rounded-full"
            disabled={selectDisabled}
            onPress={onSelect}>
            <Ionicons
              name={selectionIcon}
              size={22}
              color={
                unavailable
                  ? colors.textTertiary
                  : model.isSelected
                    ? colors.brand
                    : colors.textSecondary
              }
            />
          </Pressable>
        ) : null}

        {!editLocked ? (
          <Pressable
            className="h-10 w-10 items-center justify-center rounded-full"
            disabled={busy}
            onPress={onEdit}>
            <Ionicons name="create-outline" size={20} color={colors.textSecondary} />
          </Pressable>
        ) : null}

        <Pressable
          className="h-10 w-10 items-center justify-center rounded-full"
          disabled={deleteDisabled}
          onPress={onDelete}>
          <Ionicons
            name="trash-outline"
            size={20}
            color={deleteLocked ? colors.textTertiary : colors.brand}
          />
        </Pressable>
      </View>
    </View>
  );
}

function ModelSection({
  title,
  description,
  helper,
  addLabel,
  emptyLabel,
  models,
  busy,
  allowSelect,
  showSelectedState,
  onAdd,
  onEdit,
  onSelect,
  onDelete,
}: {
  title: string;
  description: string;
  helper?: string;
  addLabel: string;
  emptyLabel: string;
  models: UserChatModel[];
  busy: boolean;
  allowSelect: boolean;
  showSelectedState: boolean;
  onAdd: () => void;
  onEdit: (model: UserChatModel) => void;
  onSelect: (model: UserChatModel) => void;
  onDelete: (model: UserChatModel) => void;
}) {
  const {colors} = useAppTheme();
  const {t} = useI18n();

  return (
    <View className="gap-4 rounded-[24px] p-5" style={{backgroundColor: colors.surface}}>
      <View className="gap-3">
        <View className="flex-row items-start justify-between gap-3">
          <View className="flex-1 gap-2">
            <Text style={{fontSize: fontSizes.lg, fontWeight: '700', color: colors.textPrimary}}>
              {title}
            </Text>
            <Text style={{fontSize: fontSizes.sm, lineHeight: 22, color: colors.textSecondary}}>
              {description}
            </Text>
            {helper ? (
              <Text style={{fontSize: fontSizes.xs, lineHeight: 20, color: colors.textMuted}}>
                {helper}
              </Text>
            ) : null}
          </View>
          <Pressable
            className="rounded-full px-4 py-2"
            style={{backgroundColor: colors.surfaceMuted}}
            onPress={onAdd}>
            <Text style={{fontSize: fontSizes.sm, fontWeight: '600', color: colors.textPrimary}}>
              {addLabel}
            </Text>
          </Pressable>
        </View>
      </View>

      <View className="gap-3">
        {models.length === 0 ? (
          <Text style={{fontSize: fontSizes.sm, color: colors.textMuted}}>{emptyLabel}</Text>
        ) : (
          models.map((model) => (
            <ModelRow
              key={model.id}
              model={model}
              allowSelect={allowSelect}
              showSelectedState={showSelectedState}
              busy={busy}
              onEdit={() => onEdit(model)}
              onSelect={() => onSelect(model)}
              onDelete={() => onDelete(model)}
            />
          ))
        )}
      </View>
    </View>
  );
}

function CreateModelModal({
  visible,
  title,
  draft,
  busy,
  onClose,
  onChange,
  onSubmit,
}: {
  visible: boolean;
  title: string;
  draft: ModelDraft;
  busy: boolean;
  onClose: () => void;
  onChange: (field: keyof ModelDraft, value: string) => void;
  onSubmit: () => void;
}) {
  const {colors} = useAppTheme();
  const {t} = useI18n();

  return (
    <Modal transparent visible={visible} animationType="fade" onRequestClose={onClose}>
      <View className="flex-1 items-center justify-center px-5" style={{backgroundColor: colors.overlay}}>
        <Pressable className="absolute inset-0" onPress={busy ? undefined : onClose} />

        <View className="w-full max-w-[420px] gap-4 rounded-[24px] p-5" style={{backgroundColor: colors.surface}}>
          <View className="flex-row items-center justify-between gap-3">
            <Text style={{fontSize: fontSizes.lg, fontWeight: '700', color: colors.textPrimary}}>
              {title}
            </Text>
            <Pressable
              className="h-9 w-9 items-center justify-center rounded-full"
              style={{backgroundColor: colors.surfaceMuted}}
              disabled={busy}
              onPress={onClose}>
              <Ionicons name="close" size={18} color={colors.textPrimary} />
            </Pressable>
          </View>

          <View className="gap-3">
            <TextField
              label={t('models.nameLabel')}
              value={draft.name}
              onChangeText={(value) => onChange('name', value)}
              placeholder={t('models.namePlaceholder')}
            />
            <TextField
              label={t('models.baseUrlLabel')}
              value={draft.baseUrl}
              onChangeText={(value) => onChange('baseUrl', value)}
              placeholder={t('models.baseUrlPlaceholder')}
            />
            <TextField
              label={t('models.apiKeyLabel')}
              value={draft.apiKey}
              onChangeText={(value) => onChange('apiKey', value)}
              placeholder={t('models.apiKeyPlaceholder')}
              secureTextEntry
            />
            <TextField
              label={t('models.modelNameLabel')}
              value={draft.modelName}
              onChangeText={(value) => onChange('modelName', value)}
              placeholder={t('models.modelNamePlaceholder')}
            />
          </View>

          <View className="flex-row gap-3">
            <View className="flex-1">
              <PrimaryButton label={t('common.cancel')} secondary disabled={busy} onPress={onClose} />
            </View>
            <View className="flex-1">
              <PrimaryButton
                label={t('models.save')}
                disabled={
                  busy ||
                  !draft.name.trim() ||
                  !draft.baseUrl.trim() ||
                  !draft.modelName.trim()
                }
                onPress={onSubmit}
              />
            </View>
          </View>
        </View>
      </View>
    </Modal>
  );
}

const EMPTY_DRAFT: ModelDraft = {
  name: '',
  baseUrl: 'http://127.0.0.1:11434/v1',
  apiKey: 'ollama',
  modelName: '',
};

export default function ModelSettingsScreen() {
  const router = useRouter();
  const accessToken = useAuthStore((state) => state.accessToken)!;
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [createPurpose, setCreatePurpose] = useState<ChatModelPurpose | null>(null);
  const [editingTarget, setEditingTarget] = useState<UserChatModel | null>(null);
  const [draft, setDraft] = useState<ModelDraft>(EMPTY_DRAFT);
  const [deleteTarget, setDeleteTarget] = useState<UserChatModel | null>(null);
  const {showTatos, tatosNode} = useTatos();

  const modelsQuery = useQuery({
    queryKey: ['chat-models'],
    queryFn: () => api.listChatModels(accessToken),
  });

  const generalModels = modelsQuery.data?.generalModels ?? [];
  const knowledgeModels = modelsQuery.data?.knowledgeModels ?? [];
  const busy = busyKey !== null;

  const createTitle = useMemo(() => {
    if (editingTarget?.purpose === 'knowledge') {
      return t('models.editKnowledgeSheetTitle');
    }
    if (editingTarget?.purpose === 'general') {
      return t('models.editGeneralSheetTitle');
    }
    if (createPurpose === 'knowledge') {
      return t('models.addKnowledgeSheetTitle');
    }
    return t('models.addGeneralSheetTitle');
  }, [createPurpose, editingTarget?.purpose, t]);

  const createTargetModels = useMemo(() => {
    if (createPurpose === 'knowledge') {
      return knowledgeModels;
    }
    return generalModels;
  }, [createPurpose, generalModels, knowledgeModels]);

  const refreshModels = async () => {
    await queryClient.invalidateQueries({queryKey: ['chat-models']});
  };

  const openCreateModal = (purpose: ChatModelPurpose) => {
    setCreatePurpose(purpose);
    setEditingTarget(null);
    setDraft(EMPTY_DRAFT);
  };

  const openEditModal = (model: UserChatModel) => {
    setCreatePurpose(model.purpose);
    setEditingTarget(model);
    setDraft({
      name: model.name,
      baseUrl: model.baseUrl,
      apiKey: model.apiKey ?? '',
      modelName: model.modelName,
    });
  };

  const closeCreateModal = () => {
    if (busy) {
      return;
    }
    setCreatePurpose(null);
    setEditingTarget(null);
    setDraft(EMPTY_DRAFT);
  };

  const saveModel = async () => {
    if (!createPurpose) {
      return;
    }
    if (hasDuplicateModelName(createTargetModels, draft.name, editingTarget?.id)) {
      showTatos({
        title: editingTarget ? t('models.editFailed') : t('models.addFailed'),
        body: t('models.duplicate'),
      });
      return;
    }
    const actionKey = editingTarget ? `update:${editingTarget.id}` : `create:${createPurpose}`;
    setBusyKey(actionKey);
    try {
      const payload = {
        name: draft.name.trim(),
        baseUrl: draft.baseUrl.trim(),
        apiKey: draft.apiKey.trim(),
        modelName: draft.modelName.trim(),
      };
      if (editingTarget) {
        await api.updateChatModel(accessToken, editingTarget.id, payload);
      } else {
        await api.createChatModel(accessToken, {
          purpose: createPurpose,
          ...payload,
        });
      }
      await refreshModels();
      setCreatePurpose(null);
      setEditingTarget(null);
      setDraft(EMPTY_DRAFT);
    } catch (error) {
      showTatos({
        title: editingTarget ? t('models.editFailed') : t('models.addFailed'),
        body: modelMutationErrorMessage(error, t),
      });
    } finally {
      setBusyKey(null);
    }
  };

  const selectModel = async (model: UserChatModel) => {
    const actionKey = `select:${model.id}`;
    setBusyKey(actionKey);
    try {
      await api.selectChatModel(accessToken, model.id);
      await refreshModels();
    } catch (error) {
      showTatos({
        title: t('models.selectFailed'),
        body: errorMessage(error, t('models.selectFailed')),
      });
    } finally {
      setBusyKey(null);
    }
  };

  const deleteModel = async (model: UserChatModel) => {
    const actionKey = `delete:${model.id}`;
    setBusyKey(actionKey);
    try {
      await api.deleteChatModel(accessToken, model.id);
      await refreshModels();
    } catch (error) {
      showTatos({
        title: t('models.deleteFailed'),
        body: errorMessage(error, t('models.deleteFailed')),
      });
    } finally {
      setBusyKey(null);
      setDeleteTarget(null);
    }
  };

  return (
    <SafeAreaView
      className="flex-1"
      style={{backgroundColor: colors.background}}
      edges={['top', 'bottom', 'left', 'right']}>
      <View className="mb-2 flex-row items-center justify-between px-3 pt-3" style={{backgroundColor: colors.background}}>
        <Pressable className="h-11 w-11 items-center justify-center rounded-full" onPress={() => router.back()}>
          <Ionicons name="chevron-back" size={22} color={colors.textPrimary} />
        </Pressable>
        <Text style={{fontSize: fontSizes.lg, fontWeight: '500', color: colors.textPrimary}}>
          {t('models.title')}
        </Text>
        <View className="w-11" />
      </View>

      {modelsQuery.isLoading && !modelsQuery.data ? (
        <View className="flex-1 items-center justify-center gap-3 px-8">
          <ActivityIndicator size="large" color={colors.brand} />
          <Text style={{fontSize: fontSizes.sm, color: colors.textSecondary}}>
            {t('models.loading')}
          </Text>
        </View>
      ) : modelsQuery.error ? (
        <View className="flex-1 items-center justify-center gap-4 px-8">
          <Text
            className="text-center"
            style={{fontSize: fontSizes.sm, lineHeight: 22, color: colors.textSecondary}}>
            {modelSettingsErrorMessage(modelsQuery.error, t)}
          </Text>
          <View className="min-w-[140px]">
            <PrimaryButton
              label={t('models.retry')}
              onPress={() => {
                void modelsQuery.refetch();
              }}
            />
          </View>
        </View>
      ) : (
        <ScrollView
          className="flex-1"
          contentContainerStyle={{paddingHorizontal: 20, paddingBottom: 28}}
          showsVerticalScrollIndicator={false}>
          <View className="gap-5">
            <ModelSection
              title={t('models.generalTitle')}
              description={t('models.generalDescription')}
              helper={t('models.generalCurrent')}
              addLabel={t('models.addGeneral')}
              emptyLabel={t('models.emptyGeneral')}
              models={generalModels}
              busy={busy}
              allowSelect={false}
              showSelectedState={false}
              onAdd={() => openCreateModal('general')}
              onEdit={openEditModal}
              onSelect={() => {}}
              onDelete={(model) => {
                setDeleteTarget(model);
              }}
            />

            <ModelSection
              title={t('models.knowledgeTitle')}
              description={t('models.knowledgeDescription')}
              addLabel={t('models.addKnowledge')}
              emptyLabel={t('models.emptyKnowledge')}
              models={knowledgeModels}
              busy={busy}
              allowSelect
              showSelectedState
              onAdd={() => openCreateModal('knowledge')}
              onEdit={openEditModal}
              onSelect={(model) => {
                void selectModel(model);
              }}
              onDelete={(model) => {
                setDeleteTarget(model);
              }}
            />
          </View>
        </ScrollView>
      )}

      <CreateModelModal
        visible={createPurpose !== null}
        title={createTitle}
        draft={draft}
        busy={busy}
        onClose={closeCreateModal}
        onChange={(field, value) => {
          setDraft((current) => ({...current, [field]: value}));
        }}
        onSubmit={() => {
          void saveModel();
        }}
      />

      <ConfirmModal
        visible={deleteTarget !== null}
        title={t('models.deleteConfirmTitle')}
        body={t('models.deleteConfirmBody', {name: deleteTarget?.name ?? ''})}
        cancelLabel={t('common.cancel')}
        confirmLabel={t('models.delete')}
        onCancel={() => {
          if (busy) {
            return;
          }
          setDeleteTarget(null);
        }}
        onConfirm={() => {
          if (!deleteTarget) {
            return;
          }
          void deleteModel(deleteTarget);
        }}
      />

      {tatosNode}
    </SafeAreaView>
  );
}
