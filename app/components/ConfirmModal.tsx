import {Modal, Pressable, Text, View} from 'react-native';

import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

type ConfirmModalProps = {
  visible: boolean;
  title: string;
  body: string;
  cancelLabel: string;
  confirmLabel: string;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ConfirmModal({
  visible,
  title,
  body,
  cancelLabel,
  confirmLabel,
  onCancel,
  onConfirm,
}: ConfirmModalProps) {
  const {colors} = useAppTheme();

  return (
    <Modal
      transparent
      visible={visible}
      animationType="fade"
      statusBarTranslucent
      onRequestClose={onCancel}>
      <View className="flex-1 justify-center px-5" style={{backgroundColor: colors.overlay}}>
        <View className="rounded-[28px] p-6" style={{backgroundColor: colors.surface}}>
          <Text
            style={{fontSize: fontSizes.lg, fontWeight: '700', color: colors.textPrimary}}>
            {title}
          </Text>
          <Text
            className="mt-5"
            style={{
              fontSize: fontSizes.md,
              lineHeight: 32,
              color: colors.textSecondary,
            }}>
            {body}
          </Text>

          <View className="mt-7 flex-row gap-3">
            <Pressable
              className="flex-1 items-center rounded-[12px] py-4"
              style={{backgroundColor: colors.surfaceMuted}}
              onPress={onCancel}>
              <Text
                style={{fontSize: fontSizes.md, fontWeight: '600', color: colors.textPrimary}}>
                {cancelLabel}
              </Text>
            </Pressable>
            <Pressable
              className="flex-1 items-center rounded-[12px] py-4"
              style={{backgroundColor: colors.brand}}
              onPress={onConfirm}>
              <Text
                style={{fontSize: fontSizes.md, fontWeight: '600', color: colors.surface}}>
                {confirmLabel}
              </Text>
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  );
}
