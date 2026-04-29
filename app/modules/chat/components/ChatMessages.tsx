import {Ionicons} from '@expo/vector-icons';
import Markdown from 'react-native-markdown-display';
import {useMemo, useState} from 'react';
import {Pressable, ScrollView, Text, View, useWindowDimensions} from 'react-native';
import type {RefObject} from 'react';

import {SourceCard} from '@/components/SourceCard';
import type {ChatMessage} from '@/modules/chat/types';
import type {AppColors} from '@/theme/colors';
import {createMarkdownRules, createMarkdownStyles} from '@/theme/markdown';

function formatUsage(usage: ChatMessage['usage']) {
  if (!usage) {
    return '';
  }
  return `Tokens: prompt ${usage.prompt_tokens} · completion ${usage.completion_tokens} · total ${usage.total_tokens}`;
}

export function ChatMessages({
  colors,
  messages,
  scrollRef,
  assistantTitle,
  assistantBadgeLabel,
  greetingBody,
  sendingLabel,
  sourcesLabel,
  contentBottomPadding,
}: {
  colors: AppColors;
  messages: ChatMessage[];
  scrollRef: RefObject<ScrollView | null>;
  assistantTitle: string;
  assistantBadgeLabel: string;
  greetingBody: string;
  sendingLabel: string;
  sourcesLabel: string;
  contentBottomPadding: number;
}) {
  const {width: windowWidth} = useWindowDimensions();
  const markdownTableViewportWidth = Math.max(windowWidth - 72, 0);
  const markdownTableCellMinWidth = Math.max(Math.round((windowWidth - 72) * 0.65), 180);
  const markdownStyles = useMemo(
    () => {
      const baseStyles = createMarkdownStyles(colors, markdownTableCellMinWidth);
      return {
        ...baseStyles,
        body: {
          ...baseStyles.body,
          backgroundColor: 'transparent',
        },
      };
    },
    [colors, markdownTableCellMinWidth],
  );
  const [collapsedSourcesByMessage, setCollapsedSourcesByMessage] = useState<Record<string, boolean>>({});
  const markdownRules = useMemo(
    () => createMarkdownRules(markdownTableCellMinWidth, markdownTableViewportWidth),
    [markdownTableCellMinWidth, markdownTableViewportWidth],
  );

  return (
    <ScrollView
      ref={scrollRef}
      className="flex-1 px-5"
      contentContainerStyle={{
        gap: 16,
        paddingBottom: contentBottomPadding,
      }}
      keyboardShouldPersistTaps="handled"
      showsVerticalScrollIndicator={false}>
      {messages.length === 0 ? (
        <View className="gap-4 rounded-[24px] p-5" style={{backgroundColor: colors.surface}}>
          <View className="flex-row items-center gap-3">
            <Text className="text-[20px] font-semibold" style={{color: colors.textPrimary}}>
              {assistantTitle}
            </Text>
            <View
              className="rounded-[8px] px-2 py-1"
              style={{backgroundColor: colors.surfaceMuted}}>
              <Text className="text-xs font-semibold" style={{color: colors.textMuted}}>
                {assistantBadgeLabel}
              </Text>
            </View>
          </View>
          <Text className="text-[14px] leading-6" style={{color: colors.textPrimary}}>
            {greetingBody}
          </Text>
        </View>
      ) : (
        messages.map((message) => {
          const isUser = message.role === 'user';
          const sourcesCollapsed = collapsedSourcesByMessage[message.id] ?? true;

          return (
            <View key={message.id} className={isUser ? 'items-end' : 'items-stretch'}>
              <View
                className={`rounded-[12px] ${isUser ? 'py-2 px-4' : 'py-3'}`}
                style={{
                  maxWidth: isUser ? '84%' : '100%',
                  backgroundColor: isUser ? colors.surface : 'transparent',
                  shadowColor: colors.shadow,
                  shadowOffset: {width: 0, height: 4},
                  shadowOpacity: isUser ? 0.06 : 0,
                  shadowRadius: isUser ? 10 : 0,
                  elevation: isUser ? 2 : 0,
                }}>
                {!isUser ? (
                  <View className="mb-3 flex-row items-center gap-2">
                    <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
                      {assistantTitle}
                    </Text>
                    <View
                      className="min-w-0 shrink rounded-[10px] px-2 py-0.5">
                      <Text
                        className="text-[11px] font-semibold"
                        numberOfLines={1}
                        style={{color: colors.textMuted}}>
                        {assistantBadgeLabel}
                      </Text>
                    </View>
                  </View>
                ) : null}

                {isUser ? (
                  <Text className="text-[16px] leading-7" style={{color: colors.textPrimary}}>
                    {message.content || (message.state === 'streaming' ? sendingLabel : '')}
                  </Text>
                ) : message.content ? (
                  <Markdown rules={markdownRules} style={markdownStyles}>
                    {message.content}
                  </Markdown>
                ) : (
                  <Text className="text-[16px] leading-7" style={{color: colors.textPrimary}}>
                    {message.state === 'streaming' ? sendingLabel : ''}
                  </Text>
                )}

                {!isUser && message.sources?.length ? (
                  <View className="mb-2 gap-3">
                    <Pressable
                      className="flex-row items-center justify-between rounded-2xl px-1 py-1"
                      onPress={() => {
                        setCollapsedSourcesByMessage((current) => ({
                          ...current,
                          [message.id]: !sourcesCollapsed,
                        }));
                      }}>
                      <Text className="text-sm font-semibold" style={{color: colors.textPrimary}}>
                        {sourcesLabel}
                      </Text>
                      <Ionicons
                        name={sourcesCollapsed ? 'chevron-down' : 'chevron-up'}
                        size={18}
                        color={colors.textMuted}
                      />
                    </Pressable>
                    {!sourcesCollapsed
                      ? message.sources.map((source, index) => (
                          <SourceCard key={`${message.id}-${source.title}-${index}`} source={source} />
                        ))
                      : null}
                  </View>
                ) : null}

                {!isUser && message.usage ? (
                  <View
                    className="self-start rounded-[8px] px-2.5 py-1.5"
                    style={{backgroundColor: colors.surfaceMuted}}>
                    <Text className="text-[11px]" style={{color: colors.textMuted}}>
                      {formatUsage(message.usage)}
                    </Text>
                  </View>
                ) : null}
              </View>
            </View>
          );
        })
      )}
    </ScrollView>
  );
}
