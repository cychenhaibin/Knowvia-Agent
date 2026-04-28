import {Pressable, Text} from 'react-native';

import {useAppTheme} from '@/theme/useAppTheme';

export function PrimaryButton({
  label,
  onPress,
  disabled,
  small = false,
  secondary = false,
}: {
  label: string;
  onPress: () => void;
  disabled?: boolean;
  small?: boolean;
  secondary?: boolean;
}) {
  const {colors} = useAppTheme();

  return (
    <Pressable
      className={`items-center rounded-[12px] ${small ? 'py-2' : 'py-3'}`}
      style={{
        backgroundColor: disabled
          ? colors.surfaceMuted
          : secondary
            ? colors.surface
            : colors.brand,
        borderWidth: 1,
        borderColor: disabled
          ? colors.divider
          : secondary
            ? colors.brand
            : 'transparent',
        opacity: disabled ? 0.75 : 1,
      }}
      disabled={disabled}
      onPress={onPress}>
      <Text
        className={`${small ? 'text-[12px]' : 'font-semibold'}`}
        style={{
          color: disabled
            ? colors.textTertiary
            : secondary
              ? colors.brand
              : colors.surface,
        }}>
        {label}
      </Text>
    </Pressable>
  );
}
