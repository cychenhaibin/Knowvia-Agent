import {Ionicons} from '@expo/vector-icons';
import {Pressable, Text, View} from 'react-native';

import {BottomSheet} from '@/components/BottomSheet';
import {PrimaryButton} from '@/components/PrimaryButton';
import type {KnowledgeMode} from '@/modules/chat/types';
import type {KnowledgeConnection} from '@/types/api';
import type {AppColors} from '@/theme/colors';

export function KnowledgePickerSheet({
  colors,
  visible,
  title,
  doneLabel,
  detachedLabel,
  allLabel,
  knowledgeMode,
  connections,
  selectedConnectionIds,
  onClose,
  onClearKnowledge,
  onSelectAllKnowledge,
  onToggleConnection,
}: {
  colors: AppColors;
  visible: boolean;
  title: string;
  doneLabel: string;
  detachedLabel: string;
  allLabel: string;
  knowledgeMode: KnowledgeMode;
  connections: KnowledgeConnection[];
  selectedConnectionIds: string[];
  onClose: () => void;
  onClearKnowledge: () => void;
  onSelectAllKnowledge: () => void;
  onToggleConnection: (connectionId: string) => void;
}) {
  return (
    <BottomSheet visible={visible} onClose={onClose} title={title}>
      <View className="gap-3">
        <Pressable
          className="flex-row items-center justify-between rounded-[18px] px-4 py-4"
          style={{
            backgroundColor: knowledgeMode === 'none' ? colors.brandSoft : colors.surfaceMuted,
          }}
          onPress={onClearKnowledge}>
          <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
            {detachedLabel}
          </Text>
          {knowledgeMode === 'none' ? (
            <Ionicons name="checkmark" size={20} color={colors.brand} />
          ) : null}
        </Pressable>

        <Pressable
          className="flex-row items-center justify-between rounded-[18px] px-4 py-4"
          style={{
            backgroundColor: knowledgeMode === 'all' ? colors.brandSoft : colors.surfaceMuted,
          }}
          onPress={onSelectAllKnowledge}>
          <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
            {allLabel}
          </Text>
          {knowledgeMode === 'all' ? (
            <Ionicons name="checkmark" size={20} color={colors.brand} />
          ) : null}
        </Pressable>

        {connections.map((connection) => {
          const active = selectedConnectionIds.includes(connection.id);

	          return (
	            <Pressable
              key={connection.id}
              className="flex-row items-center justify-between rounded-[18px] px-4 py-4"
              style={{
                backgroundColor: active ? colors.brandSoft : colors.surfaceMuted,
              }}
              onPress={() => onToggleConnection(connection.id)}>
	              <View className="flex-1 gap-1">
	                <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
	                  {connection.name}
	                </Text>
	                <Text className="text-sm" style={{color: colors.textSecondary}}>
	                  {connection.provider === 'yuque'
	                    ? connection.yuque?.namespace
	                      ? `${connection.yuque.groupLogin} / ${connection.yuque.namespace}`
	                      : connection.yuque?.groupLogin ?? ''
	                    : `${connection.feishu?.entryType ?? 'docx'} / ${connection.feishu?.entryToken ?? ''}`}
	                </Text>
	              </View>
              {active ? <Ionicons name="checkmark" size={20} color={colors.brand} /> : null}
            </Pressable>
          );
        })}
      </View>

      <View className="mt-5">
        <PrimaryButton label={doneLabel} onPress={onClose} />
      </View>
    </BottomSheet>
  );
}
