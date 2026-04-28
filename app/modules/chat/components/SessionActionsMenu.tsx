import {AntDesign} from '@expo/vector-icons';
import {Ionicons} from '@expo/vector-icons';
import {Modal, Pressable, Text, View} from 'react-native';

import type {MenuPosition} from '@/modules/chat/types';
import type {AppColors} from '@/theme/colors';

export function SessionActionsMenu({
  colors,
  visible,
  position,
  renameLabel,
  pinLabel,
  deleteLabel,
  pinned,
  onClose,
  onRename,
  onPin,
  onDelete,
}: {
  colors: AppColors;
  visible: boolean;
  position: MenuPosition | null;
  renameLabel: string;
  pinLabel: string;
  deleteLabel: string;
  pinned: boolean;
  onClose: () => void;
  onRename: () => void;
  onPin: () => void;
  onDelete: () => void;
}) {
  if (!visible || !position) {
    return null;
  }

  return (
    <Modal
      transparent
      visible={visible}
      animationType="fade"
      statusBarTranslucent
      onRequestClose={onClose}>
      <View className="flex-1">
        <Pressable
          className="absolute inset-0"
          style={{backgroundColor: colors.overlaySoft}}
          onPress={onClose}
        />

        <View
          className="absolute overflow-hidden rounded-[28px]"
          style={{
            top: position.top,
            left: position.left,
            width: position.width,
            backgroundColor: colors.surface,
            shadowColor: colors.shadow,
            shadowOffset: {width: 0, height: 14},
            shadowOpacity: 0.11,
            shadowRadius: 24,
            elevation: 18,
          }}>
          <Pressable className="flex-row items-center gap-4 px-6 py-5" onPress={onRename}>
            <Ionicons name="create-outline" size={22} color={colors.textPrimary} />
            <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
              {renameLabel}
            </Text>
          </Pressable>

          <View className="ml-6 h-px" style={{backgroundColor: colors.divider}} />

          <Pressable className="flex-row items-center gap-4 px-6 py-5" onPress={onPin}>
            <AntDesign
              name={pinned ? 'pushpin' : 'pushpino'}
              size={20}
              color={colors.textPrimary}
            />
            <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
              {pinLabel}
            </Text>
          </Pressable>

          <View className="ml-6 h-px" style={{backgroundColor: colors.divider}} />

          <Pressable className="flex-row items-center gap-4 px-6 py-5" onPress={onDelete}>
            <Ionicons name="trash-outline" size={22} color={colors.brand} />
            <Text className="text-base font-semibold" style={{color: colors.brand}}>
              {deleteLabel}
            </Text>
          </Pressable>
        </View>
      </View>
    </Modal>
  );
}
