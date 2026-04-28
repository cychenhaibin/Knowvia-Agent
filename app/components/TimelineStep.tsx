import {Pressable, Text, View} from 'react-native';

import {StatusBadge} from '@/components/StatusBadge';
import {useAppTheme} from '@/theme/useAppTheme';
import type {RunStep} from '@/types/api';

export function TimelineStep({
  step,
  active = false,
  onPress,
}: {
  step: RunStep;
  active?: boolean;
  onPress?: () => void;
}) {
  const {colors} = useAppTheme();

  return (
    <Pressable
      className="flex-row gap-4 rounded-3xl p-4"
      onPress={onPress}
      style={({pressed}) => ({
        backgroundColor: colors.surface,
        borderWidth: active ? 1 : 0,
        borderColor: active ? colors.brand : 'transparent',
        opacity: pressed ? 0.94 : 1,
      })}>
      <View className="items-center">
        <View
          className="h-3 w-3 rounded-full"
          style={{backgroundColor: active ? colors.brand : colors.textTertiary}}
        />
        <View className="mt-1 flex-1 w-px" style={{backgroundColor: colors.border}} />
      </View>

      <View className="flex-1 gap-2 pb-2">
        <View className="flex-row items-center justify-between gap-3">
          <Text className="flex-1 text-base font-semibold" style={{color: colors.textPrimary}}>
            {step.label}
          </Text>
          <StatusBadge status={step.status} />
        </View>
        <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
          {step.summary}
        </Text>
      </View>
    </Pressable>
  );
}
