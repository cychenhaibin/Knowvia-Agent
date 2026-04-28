import type {ReactNode} from 'react';
import {Text, View} from 'react-native';

import {BottomSheet} from '@/components/BottomSheet';
import {PrimaryButton} from '@/components/PrimaryButton';
import {useAppTheme} from '@/theme/useAppTheme';

type ConnectorSheetProps = {
  visible: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  error?: string | null;
  submitLabel: string;
  submitting?: boolean;
  onSubmit: () => void;
  children: ReactNode;
};

export function ConnectorSheet({
  visible,
  onClose,
  title,
  description,
  error,
  submitLabel,
  submitting = false,
  onSubmit,
  children,
}: ConnectorSheetProps) {
  const {colors} = useAppTheme();

  return (
    <BottomSheet
      visible={visible}
      onClose={onClose}
      title={title}>
      <View className="gap-4">
        {description ? (
          <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
            {description}
          </Text>
        ) : null}
        {children}
        {error ? <Text className="text-sm text-red-600">{error}</Text> : null}
        <PrimaryButton
          label={submitLabel}
          disabled={submitting}
          onPress={onSubmit}
        />
      </View>
    </BottomSheet>
  );
}
