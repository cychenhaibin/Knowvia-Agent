import * as Haptics from 'expo-haptics';
import {AntDesign} from '@expo/vector-icons';
import {Ionicons} from '@expo/vector-icons';
import {useRef} from 'react';
import {Pressable, ScrollView, Text, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {ProfileIcon} from '@/icon';
import type {MenuAnchorRect} from '@/modules/chat/types';
import type {ChatSession} from '@/types/api';
import type {AppColors} from '@/theme/colors';

export function HistoryDrawer({
  colors,
  width,
  title,
  newChatLabel,
  startRunLabel,
  runHistoryLabel,
  emptyLabel,
  recentLabel,
  userName,
  activeSessionId,
  highlightedSessionId,
  sessions,
  onClose,
  onNewChat,
  onStartRun,
  onOpenRunHistory,
  onOpenProfile,
  onOpenSession,
  onOpenSessionActions,
  formatSessionTime,
}: {
  colors: AppColors;
  width: number;
  title: string;
  newChatLabel: string;
  startRunLabel: string;
  runHistoryLabel: string;
  emptyLabel: string;
  recentLabel: string;
  userName: string;
  activeSessionId: string | null;
  highlightedSessionId: string | null;
  sessions: ChatSession[];
  onClose: () => void;
  onNewChat: () => void;
  onStartRun: () => void;
  onOpenRunHistory: () => void;
  onOpenProfile: () => void;
  onOpenSession: (sessionId: string) => void;
  onOpenSessionActions: (session: ChatSession, anchorRect: MenuAnchorRect | null) => void;
  formatSessionTime: (session: ChatSession) => string;
}) {
  const longPressedSessionIdRef = useRef<string | null>(null);
  const sessionRowRefs = useRef<Record<string, View | null>>({});

  const openSessionActionsMenu = (session: ChatSession) => {
    const rowNode = sessionRowRefs.current[session.id];

    if (!rowNode) {
      onOpenSessionActions(session, null);
      return;
    }

    rowNode.measureInWindow((left, top, width, height) => {
      onOpenSessionActions(session, {left, top, width, height});
    });
  };

  return (
    <View
      style={{
        flex: 1,
        width,
        backgroundColor: colors.surface,
      }}>
      <SafeAreaView
        className="flex-1"
        style={{backgroundColor: colors.surface}}
        edges={['top', 'bottom', 'left']}>
        <View
          className="flex-1 px-5 pb-5 pt-3"
          style={{backgroundColor: colors.surface}}>
          <View className="mb-5 flex-row items-center justify-between">
            <Text
              className="mr-3 flex-1 text-lg font-semibold"
              numberOfLines={1}
              style={{color: colors.textPrimary}}>
              {title}
            </Text>
            <Pressable
              className="h-10 w-10 items-center justify-center rounded-full"
              style={{backgroundColor: colors.surfaceMuted}}
              onPress={onClose}>
              <Ionicons name="close" size={20} color={colors.textPrimary} />
            </Pressable>
          </View>

          <View className="mb-4 gap-2">
            <View className="flex-row gap-2">
              <Pressable
                className="flex-1 flex-row items-center gap-3 rounded-[18px] px-4 py-3"
                style={{backgroundColor: colors.surfaceMuted}}
                onPress={onStartRun}>
                <View
                  className="h-8 w-8 items-center justify-center rounded-full"
                  style={{backgroundColor: colors.surface}}>
                  <Ionicons name="rocket-outline" size={18} color={colors.brand} />
                </View>
                <Text
                  className="flex-1 text-base font-semibold"
                  numberOfLines={1}
                  style={{color: colors.textPrimary}}>
                  {startRunLabel}
                </Text>
              </Pressable>

              <Pressable
                className="flex-1 flex-row items-center gap-3 rounded-[18px] px-4 py-3"
                style={{backgroundColor: colors.surfaceMuted}}
                onPress={onOpenRunHistory}>
                <View
                  className="h-8 w-8 items-center justify-center rounded-full"
                  style={{backgroundColor: colors.surface}}>
                  <Ionicons name="albums-outline" size={18} color={colors.brand} />
                </View>
                <Text
                  className="flex-1 text-base font-semibold"
                  numberOfLines={1}
                  style={{color: colors.textPrimary}}>
                  {runHistoryLabel}
                </Text>
              </Pressable>
            </View>

            <Pressable
              className="flex-row items-center gap-3 rounded-[18px] px-4 py-3"
              style={{backgroundColor: colors.surfaceMuted}}
              onPress={onNewChat}>
              <View
                className="h-8 w-8 items-center justify-center rounded-full"
                style={{backgroundColor: colors.surface}}>
                <Ionicons name="add" size={20} color={colors.brand} />
              </View>
              <Text
                className="flex-1 text-base font-semibold"
                numberOfLines={1}
                style={{color: colors.textPrimary}}>
                {newChatLabel}
              </Text>
            </Pressable>
          </View>

          <Text className="mb-4 text-sm font-medium" style={{color: colors.textTertiary}}>
            {recentLabel}
          </Text>

          <ScrollView
            className="flex-1"
            showsVerticalScrollIndicator={false}
            contentContainerStyle={{paddingBottom: 16}}>
            {sessions.length ? (
              <View className="gap-2">
                {sessions.map((session) => {
                  const active = session.id === activeSessionId;
                  const highlighted = session.id === highlightedSessionId;
                  const pinned = session.pinned;
                  const handleOpenSession = () => {
                    if (longPressedSessionIdRef.current === session.id) {
                      longPressedSessionIdRef.current = null;
                      return;
                    }
                    onOpenSession(session.id);
                  };

                  return (
                    <View
                      key={session.id}
                      ref={(node) => {
                        sessionRowRefs.current[session.id] = node;
                      }}
                      className="flex-row items-center gap-3 rounded-[18px] px-4 py-3"
                      style={{
                        backgroundColor: active
                          ? colors.brandSoft
                          : highlighted
                            ? colors.surfaceMuted
                            : 'transparent',
                        borderRadius: 18,
                        overflow: 'hidden',
                      }}>
                      <Pressable
                        className="flex-1 gap-1"
                        delayLongPress={220}
                        onPress={handleOpenSession}
                        onLongPress={() => {
                          void Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light).catch(
                            () => {},
                          );
                          longPressedSessionIdRef.current = session.id;
                          openSessionActionsMenu(session);
                        }}>
                        <Text
                          className="text-base font-medium"
                          numberOfLines={1}
                          style={{color: colors.textPrimary}}>
                          {session.title}
                        </Text>
                        <Text className="text-sm" style={{color: colors.textSecondary}}>
                          {formatSessionTime(session)}
                        </Text>
                      </Pressable>
                        <View className="flex-row items-center gap-2">
                        {pinned ? (
                          <AntDesign name="pushpin" size={14} color={colors.brand} />
                        ) : null}
                        {active ? (
                          <Ionicons name="checkmark" size={18} color={colors.brand} />
                        ) : null}
                      </View>
                      <Pressable
                        className="h-9 w-9 items-center justify-center rounded-full"
                        onPress={() => openSessionActionsMenu(session)}>
                        <Ionicons
                          name="ellipsis-horizontal"
                          size={18}
                          color={colors.textMuted}
                        />
                      </Pressable>
                    </View>
                  );
                })}
              </View>
            ) : (
              <View
                className="rounded-[18px] px-4 py-3"
                style={{backgroundColor: colors.surfaceMuted}}>
                <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
                  {emptyLabel}
                </Text>
              </View>
            )}
          </ScrollView>

          <Pressable
            className="mt-3 flex-row items-center gap-3 rounded-[20px] px-4 py-3"
            style={{backgroundColor: colors.surfaceMuted}}
            onPress={onOpenProfile}>
            <View
              className="h-11 w-11 items-center justify-center rounded-full"
              style={{backgroundColor: colors.surface}}>
              <ProfileIcon color={colors.textPrimary} size={22} />
            </View>
            <Text
              className="flex-1 text-base font-semibold"
              numberOfLines={1}
              style={{color: colors.textPrimary}}>
              {userName}
            </Text>
            <Ionicons name="ellipsis-horizontal" size={18} color={colors.textMuted} />
          </Pressable>
        </View>
      </SafeAreaView>
    </View>
  );
}
