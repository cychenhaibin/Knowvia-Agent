import {Ionicons} from '@expo/vector-icons';
import {Pressable, Text, View} from 'react-native';

import type {AppColors} from '@/theme/colors';

export function OptionRow({
  colors,
  icon,
  title,
  description,
  note,
  onPress,
}: {
  colors: AppColors;
  icon: keyof typeof Ionicons.glyphMap;
  title: string;
  description: string;
  note?: string;
  onPress: () => void;
}) {
  return (
    <Pressable
      className="mb-3 flex-row items-start gap-3 rounded-[18px] p-4"
      style={{backgroundColor: colors.surfaceMuted}}
      onPress={onPress}>
      <View
        className="h-10 w-10 items-center justify-center rounded-full"
        style={{backgroundColor: colors.surface}}>
        <Ionicons name={icon} size={18} color={colors.textPrimary} />
      </View>
      <View className="flex-1 gap-1">
        <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
          {title}
        </Text>
        <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
          {description}
        </Text>
        {note ? (
          <Text className="text-xs leading-5" style={{color: colors.textMuted}}>
            {note}
          </Text>
        ) : null}
      </View>
      <Ionicons name="chevron-forward" size={18} color={colors.textMuted} />
    </Pressable>
  );
}
