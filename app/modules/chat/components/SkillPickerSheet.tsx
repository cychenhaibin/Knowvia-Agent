import {Ionicons} from '@expo/vector-icons';
import {Pressable, Text, View} from 'react-native';

import {BottomSheet} from '@/components/BottomSheet';
import type {SkillChoice} from '@/modules/chat/types';
import type {AppColors} from '@/theme/colors';

export function SkillPickerSheet({
  colors,
  visible,
  title,
  manageLabel,
  skills,
  selectedSkillId,
  onClose,
  onManage,
  onSelectSkill,
}: {
  colors: AppColors;
  visible: boolean;
  title: string;
  manageLabel: string;
  skills: SkillChoice[];
  selectedSkillId: string;
  onClose: () => void;
  onManage: () => void;
  onSelectSkill: (skillId: string) => void;
}) {
  return (
    <BottomSheet
      visible={visible}
      onClose={onClose}
      headerRight={
        <Pressable className="rounded-full px-3 py-1" onPress={onManage}>
          <Text className="text-sm font-semibold" style={{color: colors.brand}}>
            {manageLabel}
          </Text>
        </Pressable>
      }
      title={title}>
      <View className="gap-3">
        {skills.map((skill) => {
          const active = selectedSkillId === skill.id;

          return (
            <Pressable
              key={skill.id}
              className="flex-row items-center justify-between rounded-[18px] px-4 py-4"
              style={{
                backgroundColor: active ? colors.brandSoft : colors.surfaceMuted,
              }}
              onPress={() => onSelectSkill(skill.id)}>
              <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
                {skill.title}
              </Text>
              {active ? <Ionicons name="checkmark" size={20} color={colors.brand} /> : null}
            </Pressable>
          );
        })}
      </View>
    </BottomSheet>
  );
}
