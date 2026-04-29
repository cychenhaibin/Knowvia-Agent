import {Ionicons} from '@expo/vector-icons';
import {Pressable, Switch, Text, TextInput, View} from 'react-native';
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
  enableSearch,
  searchLabel,
  searchStateLabel,
  onInputChange,
  onSend,
  onOpenComposerMenu,
  onOpenSkillPicker,
  onOpenKnowledgePicker,
  onEnableSearchChange,
  onHeightChange,
}: {
  colors: AppColors;
  input: string;
  streaming: boolean;
  placeholder: string;
  composerBottomPadding: number;
  selectedSkillTitle?: string;
  showSkillChip: boolean;
  activeKnowledgeLabels: string[];
  enableSearch: boolean;
  searchLabel: string;
  searchStateLabel: string;
  onInputChange: (value: string) => void;
  onSend: () => void;
  onOpenComposerMenu: () => void;
  onOpenSkillPicker: () => void;
  onOpenKnowledgePicker: () => void;
  onEnableSearchChange: (value: boolean) => void;
  onHeightChange?: (height: number) => void;
}) {
  const canSend = input.trim().length > 0 && !streaming;

  return (
    <View
      className="gap-3 px-5"
      onLayout={(event) => onHeightChange?.(event.nativeEvent.layout.height)}
      style={{
        position: 'absolute',
        left: 0,
        right: 0,
        bottom: -20,
        backgroundColor: 'transparent',
        zIndex: 20,
        elevation: 20,
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

      <View
        className="rounded-[24px] px-2 pb-2 pt-4"
        style={{
          backgroundColor: colors.surface,
          shadowColor: colors.shadow,
          shadowOffset: {width: 0, height: 8},
          shadowOpacity: 0.02,
          shadowRadius: 18,
          elevation: 6,
        }}>
        <TextInput
          multiline
          value={input}
          onChangeText={onInputChange}
          placeholder={placeholder}
          placeholderTextColor={colors.textTertiary}
          style={{
            // minHeight: 48,
            maxHeight: 118,
            lineHeight: 26,
            color: colors.textPrimary,
            textAlignVertical: 'top',
            fontSize: fontSizes.md,
            paddingHorizontal: 6,
            paddingTop: 0,
            paddingBottom: 0,
            marginVertical: 0,
            includeFontPadding: false,
          }}
        />

        <View className="mt-3 flex-row items-center gap-2">
          <Pressable
            className="items-center justify-center ml-1 rounded-full"
            style={({pressed}) => ({
              backgroundColor: pressed ? colors.surfaceMuted : 'transparent',
            })}
            onPress={onOpenComposerMenu}>
            <Ionicons name="add" size={24} color={colors.textPrimary} />
          </Pressable>

          <View className="flex-row items-center">
            <Switch
              value={enableSearch}
              onValueChange={onEnableSearchChange}
              trackColor={{false: colors.surfaceMuted, true: colors.brandSoft}}
              thumbColor={enableSearch ? colors.brand : colors.textTertiary}
              ios_backgroundColor={colors.surfaceMuted}
              style={{transform: [{scaleX: 0.72}, {scaleY: 0.72}]}}
            />
            <View className="flex-row items-center">
              <Text
                className="text-[10px] font-semibold"
                numberOfLines={1}
                style={{color: enableSearch ? colors.brand : colors.textMuted}}>
                {searchLabel}
              </Text>
              <Text
                className="text-[10px]"
                numberOfLines={1}
                style={{color: colors.textTertiary}}>
                {searchStateLabel}
              </Text>
            </View>
          </View>

          <View className="flex-1" />

          {/* <View
            className="h-11 w-11 items-center justify-center rounded-full"
            style={{borderColor: colors.border, borderWidth: 1}}>
            <Ionicons name="chatbubble-ellipses-outline" size={24} color={colors.icon} />
          </View>

          <View className="h-11 w-11 items-center justify-center rounded-full">
            <Ionicons name="mic-outline" size={28} color={colors.icon} />
          </View> */}

          <Pressable
            className="h-10 w-10 items-center justify-center rounded-full"
            style={{
              backgroundColor: canSend ? colors.brand : colors.surfaceMuted,
            }}
            disabled={!canSend}
            onPress={onSend}>
            <Ionicons
              name="arrow-up"
              size={20}
              color={canSend ? colors.surface : colors.textTertiary}
            />
          </Pressable>
        </View>
      </View>
    </View>
  );
}
