import {Linking, Pressable, Text, View} from 'react-native';

import {useAppTheme} from '@/theme/useAppTheme';
import type {ChatSource, RunSource} from '@/types/api';

type SourcePreview =
  | Pick<RunSource, 'provider' | 'title' | 'repo' | 'url' | 'snippet' | 'score'>
  | ChatSource;

export function SourceCard({
  source,
  active = false,
  onPress,
}: {
  source: SourcePreview;
  active?: boolean;
  onPress?: () => void;
}) {
  const {colors} = useAppTheme();

  const body = (
    <View
      className="gap-2 rounded-3xl p-4"
      style={{
        backgroundColor: colors.surface,
        borderWidth: active ? 2 : 0,
        borderColor: active ? colors.brand : 'transparent',
      }}>
      <View className="flex-row items-center justify-between">
        <Text className="text-xs uppercase" style={{color: colors.textTertiary}}>
          {source.provider}
        </Text>
        <Text className="text-xs" style={{color: colors.textTertiary}}>
          {source.score.toFixed(1)}
        </Text>
      </View>
      <View className="flex-row items-start gap-3">
        <View className="flex-1 gap-2">
          <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
            {source.title}
          </Text>
          {source.repo ? (
            <Text className="text-sm" style={{color: colors.textSecondary}}>
              {source.repo}
            </Text>
          ) : null}
        </View>
      </View>
      <View className="gap-2">
        <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
          {source.snippet}
        </Text>
        {'matched_answer_lines' in source && source.matched_answer_lines?.length ? (
          <View className="gap-1 rounded-2xl p-3" style={{backgroundColor: colors.surfaceMuted}}>
            <Text className="text-xs font-semibold" style={{color: colors.textMuted}}>
              命中的回答句子
            </Text>
            {source.matched_answer_lines.map((line, index) => (
              <Text key={`${source.title}-line-${index}`} className="text-sm leading-6" style={{color: colors.textPrimary}}>
                {line}
              </Text>
            ))}
          </View>
        ) : null}
        {source.url ? (
          <Pressable onPress={() => Linking.openURL(source.url!)}>
            <Text className="text-sm font-medium" style={{color: colors.brand}}>
              {source.url}
            </Text>
          </Pressable>
        ) : null}
      </View>
    </View>
  );

  if (onPress) {
    return <Pressable onPress={onPress}>{body}</Pressable>;
  }

  return body;
}
