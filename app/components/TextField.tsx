import {Text, TextInput, View} from 'react-native';

import {useAppTheme} from '@/theme/useAppTheme';

export function TextField({
  label,
  value,
  onChangeText,
  placeholder,
  multiline = false,
  secureTextEntry = false,
  editable = true,
}: {
  label: string;
  value: string;
  onChangeText: (value: string) => void;
  placeholder: string;
  multiline?: boolean;
  secureTextEntry?: boolean;
  editable?: boolean;
}) {
  const {colors} = useAppTheme();

  return (
    <View className="gap-2">
      <Text className="text-md font-semibold" style={{color: colors.textPrimary}}>
        {label}
      </Text>
      <TextInput
        className={`rounded-xl px-4 py-3 text-md ${multiline ? 'min-h-36' : ''}`}
        style={{
          borderWidth: 1,
          borderColor: colors.border,
          backgroundColor: colors.surface,
          color: colors.textPrimary,
        }}
        multiline={multiline}
        placeholder={placeholder}
        placeholderTextColor={colors.textTertiary}
        secureTextEntry={secureTextEntry}
        editable={editable}
        textAlignVertical={multiline ? 'top' : 'center'}
        value={value}
        onChangeText={onChangeText}
      />
    </View>
  );
}
