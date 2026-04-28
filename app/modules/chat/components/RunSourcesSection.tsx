import {Pressable, Text, View} from 'react-native';

import {SourceCard} from '@/components/SourceCard';
import {useI18n} from '@/i18n/useI18n';
import {useAppTheme} from '@/theme/useAppTheme';
import type {RunArtifact, RunSource} from '@/types/api';

import {sourceIdentity} from '../utils/runGrounding';

type ArtifactFocus = RunArtifact['kind'] | null;

export function RunSourcesSection({
  sources,
  groundedSourceKeys,
  groundedSourceCount,
  filteredSources,
  focusedArtifactKind,
  selectedGroundedSourceKey,
  showGroundedSourcesOnly,
  groundedSourceFilterOrigin,
  onShowAllSources,
  onShowGroundedSources,
  onSourceLayout,
  onSelectGroundedSource,
}: {
  sources: RunSource[];
  groundedSourceKeys: Set<string>;
  groundedSourceCount: number;
  filteredSources: RunSource[];
  focusedArtifactKind: ArtifactFocus;
  selectedGroundedSourceKey: string | null;
  showGroundedSourcesOnly: boolean;
  groundedSourceFilterOrigin: 'auto' | 'manual' | 'none';
  onShowAllSources: () => void;
  onShowGroundedSources: () => void;
  onSourceLayout: (key: string, y: number) => void;
  onSelectGroundedSource: (key: string) => void;
}) {
  const {colors} = useAppTheme();
  const {t} = useI18n();

  return (
    <View className="gap-3">
      <View
        className="flex-row items-start justify-between gap-3 rounded-[24px] p-4"
        style={{
          backgroundColor: colors.surface,
          borderWidth: 1,
          borderColor: colors.border,
        }}>
        <View className="flex-1 gap-1">
          <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
            {t('detail.sources')}
          </Text>
          {groundedSourceKeys.size > 0 ? (
            <View className="gap-1">
              <Text className="text-xs" style={{color: colors.textTertiary}}>
                {t('detail.sourcesGroundedCount', {
                  grounded: String(groundedSourceCount),
                  total: String(sources.length),
                })}
              </Text>
              {showGroundedSourcesOnly && groundedSourceFilterOrigin === 'auto' ? (
                <Text className="text-[11px]" style={{color: colors.brand}}>
                  {t('detail.sourcesAutoGroundedHint')}
                </Text>
              ) : null}
            </View>
          ) : null}
        </View>
        {groundedSourceKeys.size > 0 ? (
          <View
            className="flex-row rounded-full p-1"
            style={{backgroundColor: colors.surfaceMuted, borderWidth: 1, borderColor: colors.border}}>
            <Pressable
              className="rounded-full px-3 py-2"
              onPress={onShowAllSources}
              style={{
                backgroundColor: showGroundedSourcesOnly ? 'transparent' : colors.surface,
                borderWidth: showGroundedSourcesOnly ? 0 : 1,
                borderColor: showGroundedSourcesOnly ? 'transparent' : colors.brand,
              }}>
              <Text className="text-xs font-semibold" style={{color: colors.textPrimary}}>
                {t('detail.sourcesShowAll')}
              </Text>
            </Pressable>
            <Pressable
              className="rounded-full px-3 py-2"
              onPress={onShowGroundedSources}
              style={{
                backgroundColor: showGroundedSourcesOnly ? colors.surface : 'transparent',
                borderWidth: showGroundedSourcesOnly ? 1 : 0,
                borderColor: showGroundedSourcesOnly ? colors.brand : 'transparent',
              }}>
              <Text className="text-xs font-semibold" style={{color: colors.textPrimary}}>
                {t('detail.sourcesShowGrounded')}
              </Text>
            </Pressable>
          </View>
        ) : null}
      </View>
      {filteredSources.length ? (
        filteredSources.map((source) => {
          const key = sourceIdentity(source);
          return (
            <View
              key={source.id}
              onLayout={(event) => {
                onSourceLayout(key, event.nativeEvent.layout.y);
              }}>
              <SourceCard
                source={source}
                active={
                  key === selectedGroundedSourceKey ||
                  (focusedArtifactKind === 'report_grounding' && groundedSourceKeys.has(key))
                }
                onPress={() => {
                  if (!groundedSourceKeys.has(key)) {
                    return;
                  }
                  onSelectGroundedSource(key);
                }}
              />
            </View>
          );
        })
      ) : (
        <View className="rounded-[28px] p-5" style={{backgroundColor: colors.surface}}>
          <Text className="text-sm" style={{color: colors.textSecondary}}>
            {t('detail.noSources')}
          </Text>
        </View>
      )}
    </View>
  );
}
