import { Ionicons } from '@expo/vector-icons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import Markdown from 'react-native-markdown-display';
import { useEffect, useMemo, useState } from 'react';
import {
  Pressable,
  ScrollView,
  Switch,
  Text,
  TextInput,
  useWindowDimensions,
  View,
} from 'react-native';

import { ConfirmModal } from '@/components/ConfirmModal';
import { PrimaryButton } from '@/components/PrimaryButton';
import { Screen } from '@/components/Screen';
import { useTatos } from '@/components/Tatos';
import { useI18n } from '@/i18n/useI18n';
import type { MessageKey } from '@/i18n/messages';
import {
  buildPromptPreview,
  formatSkillActionError,
  prefersPromptPreview,
} from '@/lib/skills';
import { type SkillSource, useSkillsStore } from '@/store/skills';
import { createMarkdownRules, createMarkdownStyles } from '@/theme/markdown';
import { fontSizes } from '@/theme/typography';
import { useAppTheme } from '@/theme/useAppTheme';
import type { ChatSkill } from '@/types/api';

type PromptViewMode = 'preview' | 'source';

const modeOptions: ChatSkill[] = ['answer', 'summary', 'actions'];

function sourceToLabel(source: SkillSource, t: (key: any, params?: any) => string) {
  const map = {
    official: t('skills.source.official'),
    manual: t('skills.source.manual'),
    github: t('skills.source.github'),
    upload: t('skills.source.upload'),
  } as const;
  return map[source];
}

export default function SkillEditorScreen() {
  const router = useRouter();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { colors } = useAppTheme();
  const { t } = useI18n();
  const { width: windowWidth } = useWindowDimensions();
  const installedSkills = useSkillsStore((state) => state.installedSkills);
  const updateSkill = useSkillsStore((state) => state.updateSkill);
  const removeSkill = useSkillsStore((state) => state.removeSkill);
  const skill = installedSkills.find((item) => item.id === id);

  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [prompt, setPrompt] = useState('');
  const [repoUrl, setRepoUrl] = useState('');
  const [mode, setMode] = useState<ChatSkill>('answer');
  const [enabled, setEnabled] = useState(true);
  const [promptViewMode, setPromptViewMode] = useState<PromptViewMode>('source');
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [busy, setBusy] = useState(false);
  const { showTatos, tatosNode } = useTatos();

  useEffect(() => {
    if (!skill) {
      return;
    }
    setTitle(skill.title);
    setDescription(skill.description);
    setPrompt(skill.prompt);
    setRepoUrl(skill.repoUrl ?? '');
    setMode(skill.mode);
    setEnabled(skill.enabled);
    setPromptViewMode(prefersPromptPreview(skill.prompt) ? 'preview' : 'source');
  }, [skill?.id]);

  const promptPreview = useMemo(() => buildPromptPreview(prompt), [prompt]);
  const markdownTableViewportWidth = Math.max(windowWidth - 88, 0);
  const markdownTableCellMinWidth = Math.max(Math.round((windowWidth - 88) * 0.65), 180);

  const markdownStyles = useMemo(
    () => createMarkdownStyles(colors, markdownTableCellMinWidth),
    [colors, markdownTableCellMinWidth],
  );

  const markdownRules = useMemo(
    () => createMarkdownRules(markdownTableCellMinWidth, markdownTableViewportWidth),
    [markdownTableCellMinWidth, markdownTableViewportWidth],
  );

  const saveSkillChanges = async () => {
    if (!skill || !title.trim() || !prompt.trim() || busy) {
      return;
    }
    setBusy(true);
    try {
      await updateSkill(skill.id, {
        title: title.trim(),
        description: description.trim(),
        prompt: prompt.trim(),
        mode,
        enabled,
        repoUrl: skill.source === 'github' ? repoUrl.trim() || undefined : undefined,
      });
      router.replace('/skills');
    } catch (error) {
      showTatos({
        body:
          error instanceof Error && error.message
            ? formatSkillActionError(error.message, t)
            : t('status.failed'),
      });
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = () => {
    if (!skill || busy) {
      return;
    }
    setShowDeleteConfirm(true);
  };

  if (!skill) {
    return (
      <Screen>
        <View className="flex-1 gap-6">
          <View className="flex-row items-center gap-3">
            <Pressable
              className="h-10 w-10 items-center justify-center rounded-full"
              style={{ backgroundColor: colors.surface }}
              onPress={() => router.replace('/skills')}>
              <Ionicons name="arrow-back" size={20} color={colors.textPrimary} />
            </Pressable>
            <Text className="text-lg font-semibold" style={{ color: colors.textPrimary }}>
              {t('skills.editSkillTitle')}
            </Text>
          </View>
          <View
            className="rounded-[24px] p-5"
            style={{ backgroundColor: colors.surface }}>
            <Text className="text-base" style={{ color: colors.textSecondary }}>
              {t('skills.notFound')}
            </Text>
          </View>
        </View>
      </Screen>
    );
  }

  return (
    <>
      <Screen>
        <View className="flex-1 gap-6">
          <View className="flex-row items-center gap-3">
            <Pressable
              className="h-10 w-10 items-center justify-center rounded-full"
              style={{ backgroundColor: colors.surface }}
              onPress={() => router.back()}>
              <Ionicons name="arrow-back" size={20} color={colors.textPrimary} />
            </Pressable>
            <View className="flex-1">
              <Text className="text-lg font-semibold" style={{ color: colors.textPrimary }}>
                {t('skills.editSkillTitle')}
              </Text>
              <Text className="text-sm" style={{ color: colors.textSecondary }}>
                {skill.title}
              </Text>
            </View>
          </View>

          <ScrollView
            className="flex-1"
            showsVerticalScrollIndicator={false}>
            <View className="gap-6">
              <View className="rounded-[12px] p-5" style={{ backgroundColor: colors.surface }}>
                <Text className="text-xl font-bold" style={{ color: colors.textPrimary }}>
                  {skill.title}
                </Text>
                <Text className="mt-2 text-sm" style={{ color: colors.textSecondary }}>
                  {sourceToLabel(skill.source, t)} ·{' '}
                  {t('skills.updatedAt', {
                    date: new Date(skill.updatedAt).toLocaleDateString(),
                  })}
                </Text>
                <View className="mt-2 flex-row items-center justify-between">
                  <Text style={{ fontSize: fontSizes.sm, color: colors.textPrimary }}>
                    {t('skills.enabledToggle')}
                  </Text>
                  <Switch
                    value={enabled}
                    onValueChange={setEnabled}
                    trackColor={{
                      false: colors.surfaceMuted,
                      true: colors.brand,
                    }}
                    thumbColor={colors.surface}
                  />
                </View>
              </View>

              <View className="rounded-[12px] p-4" style={{ backgroundColor: colors.surfaceMuted }}>
                <Text className="font-semibold" style={{ color: colors.textPrimary }}>
                  {t('skills.formName')}
                </Text>
                <TextInput
                  value={title}
                  onChangeText={setTitle}
                  placeholder={t('skills.formName')}
                  placeholderTextColor={colors.textTertiary}
                  style={{ fontSize: fontSizes.sm, color: colors.textPrimary }}
                />
              </View>

              <View className="rounded-[12px] p-4" style={{ backgroundColor: colors.surfaceMuted }}>
                <Text className="font-semibold" style={{ color: colors.textPrimary }}>
                  {t('skills.formDescription')}
                </Text>
                <TextInput
                  value={description}
                  onChangeText={setDescription}
                  placeholder={t('skills.formDescription')}
                  placeholderTextColor={colors.textTertiary}
                  multiline
                  style={{
                    // minHeight: 88,
                    fontSize: fontSizes.md,
                    color: colors.textPrimary,
                    textAlignVertical: 'top',
                  }}
                />
              </View>

              <View className="rounded-[12px] p-4" style={{ backgroundColor: colors.surfaceMuted }}>
                <View className="mb-3 flex-row items-center justify-between gap-3">
                  <Text className="font-semibold" style={{ color: colors.textPrimary }}>
                    {t('skills.formPrompt')}
                  </Text>
                  <View className="flex-row items-center gap-2">
                    {([
                      ['preview', t('skills.promptView.preview')],
                      ['source', t('skills.promptView.source')],
                    ] as const).map(([option, label]) => {
                      const active = promptViewMode === option;
                      return (
                        <Pressable
                          key={option}
                          className="rounded-full px-3 py-1.5"
                          style={{
                            backgroundColor: active ? colors.brandSoft : colors.surface,
                          }}
                          onPress={() => setPromptViewMode(option)}>
                          <Text
                            style={{
                              fontSize: fontSizes.xs,
                              fontWeight: active ? '600' : '500',
                              color: active ? colors.brand : colors.textSecondary,
                            }}>
                            {label}
                          </Text>
                        </Pressable>
                      );
                    })}
                  </View>
                </View>

                {promptViewMode === 'preview' ? (
                  <View
                    className="rounded-[18px] px-4 py-3"
                    style={{ backgroundColor: colors.surface, overflow: 'hidden' }}>
                    {promptPreview.hasFrontMatter ? (
                      <Text
                        className="mb-3"
                        style={{
                          fontSize: fontSizes.xs,
                          color: colors.textTertiary,
                        }}>
                        {t('skills.promptFrontMatterHidden')}
                      </Text>
                    ) : null}
                    {promptPreview.content ? (
                      <View style={{ width: '100%', flexShrink: 1 }}>
                        <Markdown rules={markdownRules} style={markdownStyles}>
                          {/* <Markdown rules={markdownRules} > */}
                          {promptPreview.content}
                        </Markdown>
                      </View>
                    ) : (
                      <Text style={{ fontSize: fontSizes.sm, color: colors.textTertiary }}>
                        {t('skills.formPrompt')}
                      </Text>
                    )}
                  </View>
                ) : (
                  <TextInput
                    value={prompt}
                    onChangeText={setPrompt}
                    placeholder={t('skills.formPrompt')}
                    placeholderTextColor={colors.textTertiary}
                    multiline
                    style={{
                      minHeight: 320,
                      fontSize: fontSizes.md,
                      color: colors.textPrimary,
                      textAlignVertical: 'top',
                    }}
                  />
                )}
              </View>

              {skill.source === 'github' ? (
                <View className="rounded-[12px] p-4" style={{ backgroundColor: colors.surfaceMuted }}>
                  <Text className="mb-2 font-semibold" style={{ color: colors.textPrimary }}>
                    {t('skills.formRepoUrl')}
                  </Text>
                  <TextInput
                    value={repoUrl}
                    onChangeText={setRepoUrl}
                    placeholder={t('skills.formGithubPlaceholder')}
                    placeholderTextColor={colors.textTertiary}
                    style={{ fontSize: fontSizes.md, color: colors.textPrimary }}
                  />
                </View>
              ) : null}

              <View className="gap-2">
                <Text className="font-semibold" style={{ color: colors.textPrimary }}>
                  {t('skills.formMode')}
                </Text>
                <View className="flex-row flex-wrap gap-2">
                  {modeOptions.map((option) => {
                    const active = mode === option;
                    return (
                      <Pressable
                        key={option}
                        className="rounded-full px-4 py-2"
                        style={{
                          backgroundColor: active ? colors.brandSoft : colors.surfaceMuted,
                        }}
                        onPress={() => setMode(option)}>
                        <Text
                          className={active ? 'font-semibold' : undefined}
                          style={{ color: active ? colors.brand : colors.textPrimary }}>
                          {t(`skills.mode.${option}` as MessageKey)}
                        </Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>

              <View className="gap-3">
                <PrimaryButton
                  label={t('skills.saveChanges')}
                  disabled={!title.trim() || !prompt.trim() || busy}
                  onPress={() => void saveSkillChanges()}
                />
                <Pressable
                  className="items-center rounded-[12px] py-3"
                  style={{
                    backgroundColor: colors.surface,
                    borderWidth: 1,
                    borderColor: colors.brand,
                    opacity: busy ? 0.7 : 1,
                  }}
                  disabled={busy}
                  onPress={handleDelete}>
                  <Text className="font-semibold" style={{ color: colors.brand }}>
                    {t('skills.deleteSkill')}
                  </Text>
                </Pressable>
              </View>
            </View>
          </ScrollView>
        </View>
      </Screen>

      <ConfirmModal
        visible={showDeleteConfirm}
        title={t('skills.deleteSkillConfirmTitle')}
        body={t('skills.deleteSkillConfirmBody', { name: skill.title })}
        cancelLabel={t('common.cancel')}
        confirmLabel={t('skills.deleteSkill')}
        onCancel={() => setShowDeleteConfirm(false)}
        onConfirm={() => {
          setShowDeleteConfirm(false);
          void (async () => {
            setBusy(true);
            try {
              await removeSkill(skill.id);
              router.replace('/skills');
            } catch (error) {
              showTatos({
                body:
                  error instanceof Error && error.message
                    ? formatSkillActionError(error.message, t)
                    : t('status.failed'),
              });
            } finally {
              setBusy(false);
            }
          })();
        }}
      />

      {tatosNode}
    </>
  );
}
