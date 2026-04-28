import {Ionicons} from '@expo/vector-icons';
import {useRouter} from 'expo-router';
import {useMemo, useState} from 'react';
import {
  Pressable,
  ScrollView,
  Switch,
  Text,
  TextInput,
  View,
} from 'react-native';

import {BottomSheet} from '@/components/BottomSheet';
import {PrimaryButton} from '@/components/PrimaryButton';
import {Screen} from '@/components/Screen';
import {useTatos} from '@/components/Tatos';
import {useI18n} from '@/i18n/useI18n';
import type {MessageKey} from '@/i18n/messages';
import {formatSkillActionError} from '@/lib/skills';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';
import {
  type InstalledSkill,
  type SkillSource,
  useSkillsStore,
} from '@/store/skills';
import type {ChatSkill} from '@/types/api';

type AddAction = 'manual' | 'upload' | 'github' | null;
type ParsedGitHubRepo = {
  owner: string;
  repo: string;
  slug: string;
  url: string;
  ref?: string;
  path?: string;
};

const modeOptions: ChatSkill[] = ['answer', 'summary', 'actions'];

function slugify(value: string) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function dirname(path: string): string | undefined {
  const parts = path.split('/').filter(Boolean);
  if (parts.length <= 1) {
    return undefined;
  }
  parts.pop();
  return parts.join('/');
}

function normalizeRepoPath(path: string | undefined): string | undefined {
  const normalized = path?.trim().replace(/^\/+|\/+$/g, '');
  return normalized ? normalized : undefined;
}

function parseGitHubRepo(url: string): ParsedGitHubRepo | null {
  const trimmed = url.trim();
  if (!trimmed) {
    return null;
  }

  let candidate = trimmed;
  if (/^git@github\.com:/i.test(candidate)) {
    candidate = `https://github.com/${candidate.replace(/^git@github\.com:/i, '')}`;
  } else if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(candidate) && /^github\.com\//i.test(candidate)) {
    candidate = `https://${candidate}`;
  }

  let parsed: URL;
  try {
    parsed = new URL(candidate);
  } catch {
    return null;
  }

  const hostname = parsed.hostname.toLowerCase();
  if (hostname !== 'github.com' && hostname !== 'www.github.com') {
    return null;
  }

  const segments = parsed.pathname.split('/').filter(Boolean);
  if (segments.length < 2) {
    return null;
  }

  const owner = segments[0];
  const repo = segments[1].replace(/\.git$/i, '');
  if (!owner || !repo) {
    return null;
  }

  let ref: string | undefined;
  let path: string | undefined;
  if ((segments[2] === 'tree' || segments[2] === 'blob') && segments.length >= 4) {
    ref = decodeURIComponent(segments[3]);
    const rawPath = segments.slice(4).map(decodeURIComponent).join('/');
    path =
      segments[2] === 'blob'
        ? normalizeRepoPath(dirname(rawPath) ?? undefined)
        : normalizeRepoPath(rawPath);
  }

  return {
    owner,
    repo,
    slug: `${owner}-${repo}`.toLowerCase(),
    url: `https://github.com/${owner}/${repo}`,
    ref,
    path,
  };
}

function sourceToLabel(source: SkillSource, t: (key: any, params?: any) => string) {
  const map = {
    official: t('skills.source.official'),
    manual: t('skills.source.manual'),
    github: t('skills.source.github'),
    upload: t('skills.source.upload'),
  } as const;
  return map[source];
}

export default function SkillsScreen() {
  const router = useRouter();
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const installedSkills = useSkillsStore((state) => state.installedSkills);
  const addSkill = useSkillsStore((state) => state.addSkill);
  const importGitHubSkill = useSkillsStore((state) => state.importGitHubSkill);
  const toggleSkill = useSkillsStore((state) => state.toggleSkill);
  const [query, setQuery] = useState('');
  const [enabledOnly, setEnabledOnly] = useState(false);
  const [showAddSheet, setShowAddSheet] = useState(false);
  const [addAction, setAddAction] = useState<AddAction>(null);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [prompt, setPrompt] = useState('');
  const [repoUrl, setRepoUrl] = useState('');
  const [mode, setMode] = useState<ChatSkill>('answer');
  const [addActionBusy, setAddActionBusy] = useState(false);
  const [togglingSkillIds, setTogglingSkillIds] = useState<string[]>([]);
  const {showTatos, tatosNode} = useTatos();

  const filteredSkills = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    return installedSkills.filter((skill) => {
      if (enabledOnly && !skill.enabled) {
        return false;
      }
      if (!keyword) {
        return true;
      }
      return (
        skill.title.toLowerCase().includes(keyword) ||
        skill.description.toLowerCase().includes(keyword)
      );
    });
  }, [enabledOnly, installedSkills, query]);

  const resetForm = () => {
    setName('');
    setDescription('');
    setPrompt('');
    setRepoUrl('');
    setMode('answer');
  };

  const openAction = (nextAction: AddAction) => {
    setShowAddSheet(false);
    resetForm();
    setAddActionBusy(false);
    setAddAction(nextAction);
  };

  const openSkillEditor = (skill: InstalledSkill) => {
    router.push({
      pathname: '/skill/[id]',
      params: {id: skill.id},
    });
  };

  const saveSkill = async () => {
    if (!name.trim() || !prompt.trim() || !addAction || addActionBusy) {
      return;
    }

    const sourceMap: Record<Exclude<AddAction, null>, SkillSource> = {
      manual: 'manual',
      upload: 'upload',
      github: 'github',
    };

    setAddActionBusy(true);
    try {
      await addSkill({
        slug: slugify(name) || `${Date.now()}`,
        title: name.trim(),
        description: description.trim() || prompt.trim(),
        prompt: prompt.trim(),
        mode,
        source: sourceMap[addAction],
        repoUrl: repoUrl.trim() || undefined,
      });
      setAddAction(null);
      resetForm();
    } catch (error) {
      showTatos({
        body:
          error instanceof Error && error.message
            ? formatSkillActionError(error.message, t)
            : t('status.failed'),
      });
    } finally {
      setAddActionBusy(false);
    }
  };
  const githubRepo = useMemo(() => parseGitHubRepo(repoUrl), [repoUrl]);
  const githubUrlError = repoUrl.trim() && !githubRepo ? t('skills.invalidGithubUrl') : null;

  const handleToggleSkill = async (skillId: string) => {
    if (togglingSkillIds.includes(skillId)) {
      return;
    }

    setTogglingSkillIds((current) => [...current, skillId]);
    try {
      await toggleSkill(skillId);
    } catch (error) {
      showTatos({
        body:
          error instanceof Error && error.message
            ? formatSkillActionError(error.message, t)
            : t('status.failed'),
      });
    } finally {
      setTogglingSkillIds((current) => current.filter((item) => item !== skillId));
    }
  };

  const submitGitHubImport = async () => {
    if (!githubRepo || addActionBusy) {
      return;
    }

    setAddActionBusy(true);
    try {
      await importGitHubSkill({
        repoUrl: githubRepo.url,
        ref: githubRepo.ref,
        path: githubRepo.path,
      });
      setAddAction(null);
      resetForm();
    } catch (error) {
      showTatos({
        body:
          error instanceof Error && error.message
            ? formatSkillActionError(error.message, t)
            : t('status.failed'),
      });
    } finally {
      setAddActionBusy(false);
    }
  };

  return (
    <>
      <Screen>
        <View className="flex-1 gap-4">
          <View
            className="flex-row items-center justify-between">
            <Pressable
              className="h-11 w-11 items-left justify-center rounded-full"
              onPress={() => router.back()}>
              <Ionicons name="chevron-back" size={22} color={colors.textPrimary} />
            </Pressable>
            <Text
              style={{fontSize: fontSizes.lg, fontWeight: '500', color: colors.textPrimary}}>
              {t('skills.title')}
            </Text>
            <Pressable
              className="h-11 w-11 items-center justify-center rounded-full"
              onPress={() => setShowAddSheet(true)}>
              <Ionicons name="add" size={28} color={colors.textPrimary} />
            </Pressable>
          </View>

          <View className="flex-row items-center gap-3">
            <View
              className="flex-1 flex-row items-center rounded-[12px] px-4"
              style={{backgroundColor: colors.surface}}>
              <Ionicons name="search-outline" size={20} color={colors.textMuted} />
              <TextInput
                value={query}
                onChangeText={setQuery}
                placeholder={t('skills.searchPlaceholder')}
                placeholderTextColor={colors.textTertiary}
                style={{
                  flex: 1,
                  marginLeft: 10,
                  color: colors.textPrimary,
                  fontSize: fontSizes.md,
                }}
              />
            </View>
            {/* <Pressable
              className="h-14 w-14 items-center justify-center rounded-[18px]"
              style={{backgroundColor: enabledOnly ? colors.brandSoft : colors.surface}}
              onPress={() => setEnabledOnly((current) => !current)}>
              <Ionicons
                name="filter-outline"
                size={22}
                color={enabledOnly ? colors.brand : colors.textPrimary}
              />
            </Pressable> */}
          </View>

          <ScrollView
            className="flex-1"
            contentContainerStyle={{gap: 14, paddingBottom: 24}}
            showsVerticalScrollIndicator={false}>
            {filteredSkills.length === 0 ? (
              <View
                className="rounded-[12px] p-4"
                style={{backgroundColor: colors.surface}}>
                <Text
                  style={{fontSize: fontSizes.lg, fontWeight: '500', color: colors.textPrimary}}>
                  {t('skills.emptyTitle')}
                </Text>
                <Text
                  className="mt-2"
                  style={{
                    fontSize: fontSizes.sm,
                    color: colors.textSecondary,
                  }}>
                  {t('skills.emptyDescription')}
                </Text>
              </View>
            ) : (
              filteredSkills.map((skill) => (
                <View
                  key={skill.id}
                  className="rounded-[12px] p-4"
                  style={{backgroundColor: colors.surface}}>
                  <View className="flex-row items-start gap-4">
                    <View className="flex-1 gap-2">
                      <Text
                        style={{
                          fontSize: fontSizes.lg,
                          fontWeight: '500',
                          color: colors.textPrimary,
                        }}>
                        {skill.title}
                      </Text>
                      <Text
                        style={{
                          fontSize: fontSizes.sm,
                          color: colors.textSecondary,
                        }}>
                        {skill.description}
                      </Text>
                    </View>

                    <Switch
                      value={skill.enabled}
                      disabled={togglingSkillIds.includes(skill.id)}
                      onValueChange={() => {
                        void handleToggleSkill(skill.id);
                      }}
                      trackColor={{
                        false: colors.surfaceMuted,
                        true: colors.brand,
                      }}
                      thumbColor={colors.surface}
                    />
                  </View>

                  <View className="mt-3 h-px" style={{backgroundColor: colors.divider}} />

                  <View className="mt-3 flex-row items-center justify-between">
                    <Text style={{fontSize: fontSizes.xs, color: colors.textMuted}}>
                      {sourceToLabel(skill.source, t)} ·{' '}
                      {t('skills.updatedAt', {
                        date: new Date(skill.updatedAt).toLocaleDateString(),
                      })}
                    </Text>
                    <View className="flex-row items-center gap-2">
                      <Text style={{fontSize: fontSizes.sm, color: colors.textMuted}}>
                        {t(`skills.mode.${skill.mode}` as MessageKey)}
                      </Text>
                      <Pressable
                        className="h-8 w-8 items-center justify-center rounded-full"
                        onPress={() => openSkillEditor(skill)}>
                        <Ionicons
                          name="create-outline"
                          size={18}
                          color={colors.textMuted}
                        />
                      </Pressable>
                    </View>
                  </View>
                </View>
              ))
            )}
          </ScrollView>
        </View>
      </Screen>

      <BottomSheet
        visible={showAddSheet}
        onClose={() => setShowAddSheet(false)}
        title={t('skills.addTitle')}>
        {[
          {
            icon: 'chatbubble-ellipses-outline' as const,
            title: t('skills.addWithAssistant'),
            description: t('skills.addWithAssistantHint'),
            action: 'manual' as const,
          },
          {
            icon: 'document-attach-outline' as const,
            title: t('skills.uploadSkill'),
            description: t('skills.uploadSkillHint'),
            action: 'upload' as const,
          },
          {
            icon: 'logo-github' as const,
            title: t('skills.importGithub'),
            description: t('skills.importGithubHint'),
            action: 'github' as const,
          },
        ].map((item) => (
          <Pressable
            key={item.action}
            className="mb-3 flex-row items-start gap-4 rounded-[18px] p-4"
            style={{backgroundColor: colors.surfaceMuted}}
            onPress={() => {
              openAction(item.action);
            }}>
            <View
              className="h-12 w-12 items-center justify-center rounded-[14px]"
              style={{backgroundColor: colors.surface}}>
              <Ionicons name={item.icon} size={22} color={colors.textPrimary} />
            </View>
            <View className="flex-1 gap-1">
              <Text
                style={{
                  fontSize: fontSizes.sm,
                  fontWeight: '500',
                  color: colors.textPrimary,
                }}>
                {item.title}
              </Text>
              <Text
                style={{
                  fontSize: fontSizes.xs,
                  color: colors.textSecondary,
                }}>
                {item.description}
              </Text>
            </View>
          </Pressable>
        ))}
      </BottomSheet>

      <BottomSheet visible={addAction !== null} onClose={() => setAddAction(null)}>
        <View className="mb-5 items-center">
          {addAction === 'github' ? (
            <>
              <View className="mb-2 flex-row items-center gap-5">
                <View
                  className="h-16 w-16 items-center justify-center rounded-[12px]"
                  style={{backgroundColor: colors.surface, borderWidth: 1, borderColor: colors.surfaceMuted}}>
                  <Ionicons name="logo-github" size={30} color={colors.textPrimary} />
                </View>
                <Ionicons
                  name="swap-horizontal-outline"
                  size={24}
                  color={colors.textMuted}
                />
                <View
                  className="h-16 w-16 items-center justify-center rounded-[12px]"
                  style={{backgroundColor: colors.surface, borderWidth: 1, borderColor: colors.surfaceMuted}}>
                  <Ionicons
                    name="sparkles-outline"
                    size={26}
                    color={colors.textPrimary}
                  />
                </View>
              </View>
              <Text
                className="text-center"
                style={{
                  fontSize: fontSizes.xl,
                  fontWeight: '600',
                  color: colors.textPrimary,
                }}>
                {t('skills.importGithub')}
              </Text>
              <Text
                className="mt-3 text-center"
                style={{
                  fontSize: fontSizes.sm,
                  color: colors.textSecondary,
                }}>
                {t('skills.importGithubSheetHint')}
              </Text>
            </>
          ) : (
            <Text
              style={{
                fontSize: fontSizes.xl,
                fontWeight: '700',
                color: colors.textPrimary,
              }}>
              {addAction === 'manual'
                ? t('skills.createManualTitle')
                : t('skills.createUploadTitle')}
            </Text>
          )}
        </View>

        <ScrollView
          contentContainerStyle={{gap: 14}}
          keyboardShouldPersistTaps="handled"
          showsVerticalScrollIndicator={false}>
          {addAction === 'github' ? (
            <View className="gap-2">
              <View
                className="rounded-[18px] px-4 py-3"
                style={{backgroundColor: colors.surfaceMuted}}>
                <Text className="mb-2 font-semibold" style={{color: colors.textPrimary}}>
                  {t('skills.formUrl')}
                </Text>
                <TextInput
                  value={repoUrl}
                  onChangeText={setRepoUrl}
                  placeholder={t('skills.formGithubPlaceholder')}
                  placeholderTextColor={colors.textTertiary}
                  style={{fontSize: fontSizes.md, color: colors.textPrimary}}
                />
              </View>
              {githubUrlError ? (
                <Text style={{fontSize: fontSizes.xs, color: '#dc2626'}}>
                  {githubUrlError}
                </Text>
              ) : null}
            </View>
          ) : (
            <>
              <View
                className="rounded-[18px] px-4 py-3"
                style={{backgroundColor: colors.surfaceMuted}}>
                <Text className="mb-2 font-semibold" style={{color: colors.textPrimary}}>
                  {t('skills.formName')}
                </Text>
                <TextInput
                  value={name}
                  onChangeText={setName}
                  placeholder={t('skills.formName')}
                  placeholderTextColor={colors.textTertiary}
                  style={{fontSize: fontSizes.md, color: colors.textPrimary}}
                />
              </View>

              <View
                className="rounded-[18px] px-4 py-3"
                style={{backgroundColor: colors.surfaceMuted}}>
                <Text className="mb-2 font-semibold" style={{color: colors.textPrimary}}>
                  {t('skills.formDescription')}
                </Text>
                <TextInput
                  value={description}
                  onChangeText={setDescription}
                  placeholder={t('skills.formDescription')}
                  placeholderTextColor={colors.textTertiary}
                  multiline
                  style={{
                    minHeight: 72,
                    fontSize: fontSizes.md,
                    color: colors.textPrimary,
                    textAlignVertical: 'top',
                  }}
                />
              </View>

              <View
                className="rounded-[18px] px-4 py-3"
                style={{backgroundColor: colors.surfaceMuted}}>
                <Text className="mb-2 font-semibold" style={{color: colors.textPrimary}}>
                  {t('skills.formPrompt')}
                </Text>
                <TextInput
                  value={prompt}
                  onChangeText={setPrompt}
                  placeholder={t('skills.formPrompt')}
                  placeholderTextColor={colors.textTertiary}
                  multiline
                  style={{
                    minHeight: 96,
                    fontSize: fontSizes.md,
                    color: colors.textPrimary,
                    textAlignVertical: 'top',
                  }}
                />
              </View>

              <View className="gap-2">
                <Text className="font-semibold" style={{color: colors.textPrimary}}>
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
                          style={{color: active ? colors.brand : colors.textPrimary}}>
                          {t(`skills.mode.${option}` as MessageKey)}
                        </Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>
            </>
          )}

          <PrimaryButton
            label={
              addAction === 'github' && addActionBusy
                ? t('skills.importingAction')
                : addAction === 'github'
                  ? t('skills.importAction')
                  : t('skills.saveSkill')
            }
            disabled={
              addAction === 'github'
                ? addActionBusy || !githubRepo
                : addActionBusy || !name.trim() || !prompt.trim()
            }
            onPress={() =>
              void (addAction === 'github' ? submitGitHubImport() : saveSkill())
            }
          />
        </ScrollView>
      </BottomSheet>

      {tatosNode}
    </>
  );
}
