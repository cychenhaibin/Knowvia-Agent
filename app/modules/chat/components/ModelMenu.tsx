import {Ionicons} from '@expo/vector-icons';
import {Modal, Pressable, Text, View} from 'react-native';

import type {ModelChoice, MenuPosition} from '@/modules/chat/types';
import type {EnabledFluxAModel} from '@/store/preferences';
import type {AppColors} from '@/theme/colors';

export function ModelMenu({
  colors,
  visible,
  position,
  models,
  selectedModelId,
  onClose,
  onSelect,
  fluxaModels = [],
}: {
  colors: AppColors;
  visible: boolean;
  position: MenuPosition | null;
  models: ModelChoice[];
  selectedModelId: string;
  onClose: () => void;
  onSelect: (modelId: string) => void;
  fluxaModels?: EnabledFluxAModel[];
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
          className="absolute overflow-hidden rounded-[24px]"
          style={{
            top: position.top,
            left: position.left,
            width: position.width,
            backgroundColor: colors.surface,
            shadowColor: colors.shadow,
            shadowOffset: {width: 0, height: 14},
            shadowOpacity: 0.12,
            shadowRadius: 20,
            elevation: 16,
          }}>
          {fluxaModels.length ? <View className="px-6 pb-2 pt-4"><Text className="text-[12px] font-medium" style={{color: colors.textSecondary}}>FluxA 模型</Text>{fluxaModels.map((model) => <View key={model.id} className="mt-2"><Text className="text-[16px]" style={{color: colors.textPrimary}}>{model.name}</Text><Text className="text-[12px]" style={{color: colors.textSecondary}}>{model.group}</Text></View>)}</View> : null}
          {models.map((model, index) => {
            const active = model.id === selectedModelId;
            const disabled = model.available === false;

            return (
              <View key={model.id}>
                {index > 0 ? (
                  <View className="ml-6 h-px" style={{backgroundColor: colors.divider}} />
                ) : null}
                <Pressable
                  className="flex-row items-start px-6 py-3"
                  disabled={disabled}
                  onPress={() => onSelect(model.id)}>
                  <View className="flex-1 gap-1">
                    <Text
                      className="text-[16px] font-normal"
                      style={{color: disabled ? colors.textMuted : colors.textPrimary}}>
                      {model.title}
                    </Text>
                    <Text
                      className="text-[12px] leading-6"
                      style={{color: disabled ? colors.textTertiary : colors.textSecondary}}>
                      {model.description}
                    </Text>
                  </View>
                  <View className="pt-1">
                    {active ? (
                      <Ionicons
                        name="checkmark"
                        size={22}
                        color={disabled ? colors.textTertiary : colors.brand}
                      />
                    ) : null}
                  </View>
                </Pressable>
              </View>
            );
          })}
        </View>
      </View>
    </Modal>
  );
}
