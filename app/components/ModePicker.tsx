import {Pressable, Text, View} from 'react-native';

import type {RunMode} from '@/types/api';
import {useI18n} from '@/i18n/useI18n';
import {useAppTheme} from '@/theme/useAppTheme';
import {fontSizes} from '@/theme/typography';

const modes: Array<{value: RunMode}> = [
  {value: 'auto'},
  {value: 'kb_only'},
  {value: 'web_only'},
  {value: 'hybrid'},
];

export function ModePicker({
  value,
  onChange,
}: {
  value: RunMode;
  onChange: (mode: RunMode) => void;
}) {
  const {colors} = useAppTheme();
  const {modeLabel, t} = useI18n();

  return (
    <View className="gap-3">
      <Text className="text-md font-semibold" style={{color: colors.textPrimary}}>
        {t('mode.title')}
      </Text>
      <View className="flex-row flex-wrap gap-2">
        {modes.map((mode) => {
          const active = mode.value === value;
          return (
            <Pressable
              key={mode.value}
              className="rounded-full px-4 py-2"
              style={{
                backgroundColor: active ? colors.brand : colors.surface,
                borderWidth: active ? 0 : 1,
                borderColor: active ? 'transparent' : colors.border,
              }}
              onPress={() => onChange(mode.value)}>
              <Text
                style={{fontSize: fontSizes.xs, color: active ? colors.surface : colors.textPrimary}}>
                {modeLabel(mode.value)}
              </Text>
            </Pressable>
          );
        })}
      </View>
    </View>
  );
}
