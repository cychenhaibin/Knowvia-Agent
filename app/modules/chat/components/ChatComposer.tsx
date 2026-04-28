import {Ionicons} from '@expo/vector-icons';
import {Pressable, Text, TextInput, View} from 'react-native';
import React from 'react';
import {fontSizes} from '@/theme/typography';
import type {AppColors} from '@/theme/colors';

export function ChatComposer({
  colors,
  input,
  streaming,
  placeholder,
  composerBottomPadding,
  selectedSkillTitle,
  showSkillChip,
  activeKnowledgeLabels,
  onInputChange,
  onSend,
  onOpenComposerMenu,
  onOpenSkillPicker,
  onOpenKnowledgePicker,
}: {
  colors: AppColors;
  input: string;
  streaming: boolean;
  placeholder: string;
  composerBottomPadding: number;
  selectedSkillTitle?: string;
  showSkillChip: boolean;
  activeKnowledgeLabels: string[];
  onInputChange: (value: string) => void;
  onSend: () => void;
  onOpenComposerMenu: () => void;
  onOpenSkillPicker: () => void;
  onOpenKnowledgePicker: () => void;
}) {
  return (
    <View
      className="gap-3 px-5 pt-3"
      style={{
        paddingBottom: composerBottomPadding,
        backgroundColor: colors.background,
      }}>
      {showSkillChip || activeKnowledgeLabels.length > 0 ? (
        <View className="flex-row flex-wrap gap-2">
          {showSkillChip && selectedSkillTitle ? (
            <Pressable
              className="flex-row items-center gap-2 rounded-full px-3 py-2"
              style={{backgroundColor: colors.surface}}
              onPress={onOpenSkillPicker}>
              <Ionicons name="sparkles-outline" size={14} color={colors.brand} />
              <Text className="text-sm font-medium" style={{color: colors.textPrimary}}>
                {selectedSkillTitle}
              </Text>
            </Pressable>
          ) : null}

          {activeKnowledgeLabels.map((label) => (
            <Pressable
              key={label}
              className="flex-row items-center gap-2 rounded-full px-3 py-2"
              style={{backgroundColor: colors.surface}}
              onPress={onOpenKnowledgePicker}>
              <Ionicons name="book-outline" size={14} color={colors.brand} />
              <Text className="text-sm font-medium" style={{color: colors.textPrimary}}>
                {label}
              </Text>
            </Pressable>
          ))}
        </View>
      ) : null}

      <View className="rounded-[28px] p-2" style={{backgroundColor: colors.surface}}>
        <View className="flex-row items-center gap-3">
          <Pressable
            className="h-11 w-11 items-center justify-center rounded-full"
            style={{backgroundColor: colors.surfaceMuted}}
            onPress={onOpenComposerMenu}>
            <Ionicons name="add" size={24} color={colors.textPrimary} />
          </Pressable>

          <TextInput
            multiline
            value={input}
            onChangeText={onInputChange}
            placeholder={placeholder}
            placeholderTextColor={colors.textTertiary}
            style={{
              flex: 1,
              minHeight: 22,
              maxHeight: 120,
              lineHeight: 22,
              color: colors.textPrimary,
              textAlignVertical: 'center',
              fontSize: fontSizes.md,
              paddingVertical: 0,
              marginVertical: 0,
              includeFontPadding: false,
            }}
          />

          <Pressable
            className="h-11 w-11 items-center justify-center rounded-full"
            style={{
              backgroundColor:
                !input.trim() || streaming ? colors.surfaceMuted : colors.brand,
            }}
            disabled={!input.trim() || streaming}
            onPress={onSend}>
            <Ionicons
              name="arrow-up"
              size={20}
              color={!input.trim() || streaming ? colors.textMuted : colors.surface}
            />
          </Pressable>
        </View>
      </View>
    </View>
  );
}
