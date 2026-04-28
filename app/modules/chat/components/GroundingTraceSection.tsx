import type {ComponentProps} from 'react';

import Markdown from 'react-native-markdown-display';
import {Pressable, Text, View} from 'react-native';

import {SourceCard} from '@/components/SourceCard';
import {useI18n} from '@/i18n/useI18n';
import {useAppTheme} from '@/theme/useAppTheme';
import type {RunSource} from '@/types/api';

import type {GroundedSourceRef} from '../utils/runGrounding';

type MarkdownRules = ComponentProps<typeof Markdown>['rules'];
type MarkdownStyles = ComponentProps<typeof Markdown>['style'];

export function GroundingTraceSection({
  focused,
  reportGrounding,
  groundedSourceRefs,
  selectedGroundedSourceKey,
  selectedGroundedSourcePreview,
  selectedGroundedSourceIndex,
  markdownRules,
  markdownStyles,
  onSectionLayout,
  onSelectGroundedSource,
  onStepGroundedSourcePreview,
}: {
  focused: boolean;
  reportGrounding: string;
  groundedSourceRefs: GroundedSourceRef[];
  selectedGroundedSourceKey: string | null;
  selectedGroundedSourcePreview: RunSource | null;
  selectedGroundedSourceIndex: number;
  markdownRules: MarkdownRules;
  markdownStyles: MarkdownStyles;
  onSectionLayout: (y: number) => void;
  onSelectGroundedSource: (key: string) => void;
  onStepGroundedSourcePreview: (offset: number) => void;
}) {
  const {colors} = useAppTheme();
  const {t} = useI18n();

  return (
    <View
      className="gap-3"
      onLayout={(event) => {
        onSectionLayout(event.nativeEvent.layout.y);
      }}>
      <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
        {t('detail.groundingTrace')}
      </Text>
      <View
        className="gap-3 rounded-[28px] p-5"
        style={{
          backgroundColor: colors.surface,
          borderWidth: focused ? 2 : 1,
          borderColor: focused ? colors.brand : colors.border,
        }}>
        <View className="gap-2">
          <View
            className="self-start rounded-full px-3 py-1"
            style={{
              backgroundColor: colors.surfaceMuted,
              borderWidth: 1,
              borderColor: colors.border,
            }}>
            <Text
              className="text-[11px] font-semibold uppercase tracking-[1px]"
              style={{color: colors.textPrimary}}>
              {t('detail.groundingTraceBadge')}
            </Text>
          </View>
          <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
            {t('detail.groundingTraceDescription')}
          </Text>
        </View>
        {reportGrounding ? (
          <View className="gap-3">
            {groundedSourceRefs.length ? (
              <View className="flex-row flex-wrap gap-2">
                {groundedSourceRefs.map((item, index) => (
                  <Pressable
                    key={item.key}
                    className="rounded-full px-3 py-2"
                    onPress={() => onSelectGroundedSource(item.key)}
                    style={{
                      backgroundColor:
                        selectedGroundedSourceKey === item.key ? colors.brandSoft : colors.surfaceMuted,
                      borderWidth: 1,
                      borderColor: selectedGroundedSourceKey === item.key ? colors.brand : colors.border,
                    }}>
                    <Text className="text-xs font-semibold" style={{color: colors.textPrimary}}>
                      {index + 1}. {item.title}
                    </Text>
                  </Pressable>
                ))}
              </View>
            ) : null}
            {selectedGroundedSourcePreview ? (
              <View className="gap-2">
                {groundedSourceRefs.length > 1 ? (
                  <View className="flex-row items-center justify-between">
                    <Pressable
                      className="rounded-full px-3 py-2"
                      disabled={selectedGroundedSourceIndex <= 0}
                      onPress={() => onStepGroundedSourcePreview(-1)}
                      style={{
                        backgroundColor:
                          selectedGroundedSourceIndex <= 0 ? colors.surfaceMuted : colors.surfaceMuted,
                        borderWidth: 1,
                        borderColor:
                          selectedGroundedSourceIndex <= 0 ? colors.border : colors.brand,
                        opacity: selectedGroundedSourceIndex <= 0 ? 0.55 : 1,
                      }}>
                      <Text className="text-xs font-semibold" style={{color: colors.textPrimary}}>
                        {t('detail.previewPrev')}
                      </Text>
                    </Pressable>
                    <Text className="text-xs" style={{color: colors.textTertiary}}>
                      {t('detail.previewPosition', {
                        current: String(selectedGroundedSourceIndex + 1),
                        total: String(groundedSourceRefs.length),
                      })}
                    </Text>
                    <Pressable
                      className="rounded-full px-3 py-2"
                      disabled={selectedGroundedSourceIndex >= groundedSourceRefs.length - 1}
                      onPress={() => onStepGroundedSourcePreview(1)}
                      style={{
                        backgroundColor: colors.surfaceMuted,
                        borderWidth: 1,
                        borderColor:
                          selectedGroundedSourceIndex >= groundedSourceRefs.length - 1
                            ? colors.border
                            : colors.brand,
                        opacity: selectedGroundedSourceIndex >= groundedSourceRefs.length - 1 ? 0.55 : 1,
                      }}>
                      <Text className="text-xs font-semibold" style={{color: colors.textPrimary}}>
                        {t('detail.previewNext')}
                      </Text>
                    </Pressable>
                  </View>
                ) : null}
                <SourceCard source={selectedGroundedSourcePreview} active />
              </View>
            ) : null}
            <Markdown rules={markdownRules} style={markdownStyles}>
              {reportGrounding}
            </Markdown>
          </View>
        ) : (
          <Text className="text-sm" style={{color: colors.textSecondary}}>
            {t('detail.waitingStageArtifact')}
          </Text>
        )}
      </View>
    </View>
  );
}
