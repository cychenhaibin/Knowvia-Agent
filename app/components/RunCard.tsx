import {Pressable, Text, View} from 'react-native';

import {StatusBadge} from '@/components/StatusBadge';
import {useI18n} from '@/i18n/useI18n';
import {useAppTheme} from '@/theme/useAppTheme';
import type {Run} from '@/types/api';

export function RunCard({
  run,
  onPress,
}: {
  run: Run;
  onPress: () => void;
}) {
  const {colors} = useAppTheme();
  const {modeLabel} = useI18n();

  return (
    <Pressable
      className="gap-3 rounded-3xl p-5 shadow-sm"
      style={{backgroundColor: colors.surface}}
      onPress={onPress}>
      <View className="flex-row items-start justify-between gap-3">
        <View className="flex-1 gap-2">
          <Text className="text-lg font-semibold" style={{color: colors.textPrimary}}>
            {run.title}
          </Text>
          <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
            {run.goal}
          </Text>
        </View>
        <StatusBadge status={run.status} />
      </View>

      <View className="flex-row justify-between">
        <Text className="text-xs uppercase" style={{color: colors.textTertiary}}>
          {modeLabel(run.effectiveMode)}
        </Text>
        <Text className="text-xs" style={{color: colors.textTertiary}}>
          {new Date(run.updatedAt).toLocaleString()}
        </Text>
      </View>
    </Pressable>
  );
}
