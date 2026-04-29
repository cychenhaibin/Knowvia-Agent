import {Ionicons} from '@expo/vector-icons';
import {useQuery} from '@tanstack/react-query';
import {useRouter} from 'expo-router';
import {useEffect, useMemo, useRef, useState} from 'react';
import {
  Animated,
  Keyboard,
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  Text,
  TextInput,
  useWindowDimensions,
  View,
} from 'react-native';
import {SafeAreaView, useSafeAreaInsets} from 'react-native-safe-area-context';

import {BottomSheet} from '@/components/BottomSheet';
import {ConfirmModal} from '@/components/ConfirmModal';
import {PrimaryButton} from '@/components/PrimaryButton';
import {useTatos} from '@/components/Tatos';
import {ChatComposer} from '@/modules/chat/components/ChatComposer';
import {ChatMessages} from '@/modules/chat/components/ChatMessages';
import {HistoryDrawer} from '@/modules/chat/components/HistoryDrawer';
import {KnowledgePickerSheet} from '@/modules/chat/components/KnowledgePickerSheet';
import {ModelMenu} from '@/modules/chat/components/ModelMenu';
import {OptionRow} from '@/modules/chat/components/OptionRow';
import {SessionActionsMenu} from '@/modules/chat/components/SessionActionsMenu';
import {SkillPickerSheet} from '@/modules/chat/components/SkillPickerSheet';
import {DEFAULT_SKILL_ID} from '@/modules/chat/constants';
import {useAndroidKeyboardInset} from '@/modules/chat/hooks/useAndroidKeyboardInset';
import {useRunsScreenAnimations} from '@/modules/chat/hooks/useRunsScreenAnimations';
import type {
  ChatMessage,
  KnowledgeMode,
  MenuAnchorRect,
  MenuPosition,
  ModelChoice,
  SkillChoice,
} from '@/modules/chat/types';
import {useI18n} from '@/i18n/useI18n';
import {api, subscribeChatStream} from '@/lib/api';
import {queryClient} from '@/lib/query-client';
import {useAuthStore} from '@/store/auth';
import {useSkillsStore} from '@/store/skills';
import type {ChatSession, PersistedChatMessage} from '@/types/api';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

const CHAT_REQUEST_TEMPERATURE = 0.05;

function formatTemperature(value: number) {
  if (!Number.isFinite(value)) {
    return '0';
  }
  if (value === 0) {
    return '0';
  }
  return value.toFixed(value < 1 ? 2 : 1).replace(/\.?0+$/, '');
}

export default function RunsScreen() {
  const router = useRouter();
  const accessToken = useAuthStore((state) => state.accessToken)!;
  const user = useAuthStore((state) => state.user);
  const installedSkills = useSkillsStore((state) => state.installedSkills);
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const {width: windowWidth, height: windowHeight} = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const [input, setInput] = useState('');
  const [selectedSkillId, setSelectedSkillId] = useState(DEFAULT_SKILL_ID);
  const [pendingGeneralModelId, setPendingGeneralModelId] = useState<string | null>(null);
  const [knowledgeMode, setKnowledgeMode] = useState<KnowledgeMode>('none');
  const [selectedConnectionIds, setSelectedConnectionIds] = useState<string[]>([]);
  const [enableSearch, setEnableSearch] = useState(false);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [streaming, setStreaming] = useState(false);
  const [composerHeight, setComposerHeight] = useState(0);
  const [showComposerMenu, setShowComposerMenu] = useState(false);
  const [showHistoryDrawer, setShowHistoryDrawer] = useState(false);
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [showDeleteSessionConfirm, setShowDeleteSessionConfirm] = useState(false);
  const [showRenameSession, setShowRenameSession] = useState(false);
  const [showSkillPicker, setShowSkillPicker] = useState(false);
  const [showKnowledgePicker, setShowKnowledgePicker] = useState(false);
  const [modelMenuPosition, setModelMenuPosition] = useState<MenuPosition | null>(null);
  const [sessionActionsMenuPosition, setSessionActionsMenuPosition] = useState<MenuPosition | null>(
    null,
  );
  const [sessionActionTarget, setSessionActionTarget] = useState<ChatSession | null>(null);
  const [renameSessionTitle, setRenameSessionTitle] = useState('');
  const [sessionActionBusy, setSessionActionBusy] = useState(false);
  const scrollRef = useRef<ScrollView | null>(null);
  const modelAnchorRef = useRef<View | null>(null);
  const streamCleanupRef = useRef<(() => void) | null>(null);
  const sessionHydratedRef = useRef(false);
  const profileNavigationPendingRef = useRef(false);
  const fallbackGeneralChatModels = useMemo(
    () => [
      {
        id: 'default-general-model',
        userId: user?.id ?? '',
        purpose: 'general' as const,
        origin: 'default' as const,
        name: 'Gemma 3n E4B',
        baseUrl: 'http://127.0.0.1:11434/v1',
        apiKey: 'ollama',
        modelName: 'gemma3n:e4b',
        temperature: CHAT_REQUEST_TEMPERATURE,
        isSelected: true,
        available: true,
        createdAt: '',
        updatedAt: '',
      },
    ],
    [user?.id],
  );

  const connectionsQuery = useQuery({
    queryKey: ['knowledge-connections'],
    queryFn: () => api.listConnections(accessToken),
  });

  const chatModelsQuery = useQuery({
    queryKey: ['chat-models'],
    queryFn: () => api.listChatModels(accessToken),
  });

  const sessionsQuery = useQuery({
    queryKey: ['chat-sessions'],
    queryFn: () => api.listChatSessions(accessToken),
  });

  const messagesQuery = useQuery({
    queryKey: ['chat-messages', activeSessionId],
    queryFn: () => api.listChatMessages(accessToken, activeSessionId!),
    enabled: Boolean(activeSessionId),
  });

  const connections = connectionsQuery.data?.items ?? [];
  const hasConnections = connections.length > 0;
  const sessions = useMemo(() => {
    const items = [...(sessionsQuery.data?.items ?? [])];
    items.sort((left, right) => {
      if (left.pinned !== right.pinned) {
        return left.pinned ? -1 : 1;
      }

      const leftDate = left.lastMessageAt ?? left.updatedAt ?? left.createdAt;
      const rightDate = right.lastMessageAt ?? right.updatedAt ?? right.createdAt;
      return new Date(rightDate).getTime() - new Date(leftDate).getTime();
    });
    return items;
  }, [sessionsQuery.data?.items]);
  const userName = user?.displayName || user?.username || t('runs.agentUser');
  const drawerWidth = Math.min(windowWidth * 0.82, 336);
  const keyboardInset = useAndroidKeyboardInset(insets.bottom);
  const {showTatos, tatosNode} = useTatos();
  const {
    contentTranslateX,
    drawerTranslateX,
    contentOverlayOpacity,
    contentExitOpacity,
    contentExitScale,
    contentExitTranslateY,
    runProfileExitAnimation,
    resetProfileExitAnimation,
  } = useRunsScreenAnimations({
    drawerWidth,
    showHistoryDrawer,
  });

  useEffect(() => {
    scrollRef.current?.scrollToEnd({animated: true});
  }, [messages]);

  useEffect(() => {
    return () => {
      streamCleanupRef.current?.();
    };
  }, []);

  useEffect(() => {
    if (sessionHydratedRef.current || !sessionsQuery.data?.items) {
      return;
    }
    sessionHydratedRef.current = true;
    if (!activeSessionId && sessionsQuery.data.items.length > 0) {
      setActiveSessionId(sessionsQuery.data.items[0].id);
    }
  }, [activeSessionId, sessionsQuery.data?.items]);

  useEffect(() => {
    if (!messagesQuery.data?.items) {
      return;
    }
    const nextMessages = messagesQuery.data.items.map((item: PersistedChatMessage) => ({
      id: item.id,
      role: item.role,
      content: item.content,
      sources: item.sources ?? [],
      usage: item.usage,
      state: 'done' as const,
    }));
    setMessages(nextMessages);
  }, [messagesQuery.data?.items]);

  const availableSkills = useMemo<SkillChoice[]>(
    () => [
      {
        id: DEFAULT_SKILL_ID,
        title: t('chat.skill.answer'),
        prompt: '',
        mode: 'answer',
      },
      ...installedSkills
        .filter((skill) => skill.enabled)
        .map((skill) => ({
          id: skill.id,
          title: skill.title,
          prompt: skill.prompt,
          mode: skill.mode,
        })),
    ],
    [installedSkills, t],
  );

  const selectedSkill =
    availableSkills.find((skill) => skill.id === selectedSkillId) ?? availableSkills[0];

  const generalChatModels =
    chatModelsQuery.data?.generalModels?.length
      ? chatModelsQuery.data.generalModels
      : fallbackGeneralChatModels;
  const knowledgeChatModels = chatModelsQuery.data?.knowledgeModels ?? [];

  const availableModels = useMemo<ModelChoice[]>(
    () =>
      generalChatModels.map((model) => ({
        id: model.id,
        modelName: model.modelName,
        title: model.name,
        description: `${model.modelName} · T=${formatTemperature(model.temperature)}`,
        temperature: model.temperature,
        available: model.available,
      })),
    [generalChatModels],
  );

  const selectedModelId =
    pendingGeneralModelId ??
    generalChatModels.find((model) => model.isSelected)?.id ??
    generalChatModels[0]?.id ??
    null;

  const selectedModel = availableModels.find((model) => model.id === selectedModelId) ?? {
    id: 'default-general-model',
    modelName: 'gemma3n:e4b',
    title: 'Gemma 3n E4B',
    description: `gemma3n:e4b · T=${formatTemperature(CHAT_REQUEST_TEMPERATURE)}`,
    temperature: CHAT_REQUEST_TEMPERATURE,
    available: true,
  };

  const selectedKnowledgeModel =
    knowledgeChatModels.find((model) => model.isSelected) ?? {
      id: 'default-knowledge-model',
      userId: user?.id ?? '',
      purpose: 'knowledge' as const,
      origin: 'default' as const,
      name: 'Qwen3 8B',
      baseUrl: 'http://127.0.0.1:11434/v1',
      apiKey: 'ollama',
      modelName: 'qwen3:8b',
      temperature: CHAT_REQUEST_TEMPERATURE,
      isSelected: true,
      available: true,
      createdAt: '',
      updatedAt: '',
    };

  const selectedModelTemperatureLabel = useMemo(
    () => `T=${formatTemperature(selectedModel.temperature)}`,
    [selectedModel.temperature],
  );

  useEffect(() => {
    if (!availableSkills.some((skill) => skill.id === selectedSkillId)) {
      setSelectedSkillId(DEFAULT_SKILL_ID);
    }
  }, [availableSkills, selectedSkillId]);

  useEffect(() => {
    if (!pendingGeneralModelId) {
      return;
    }
    if (generalChatModels.some((model) => model.id === pendingGeneralModelId && model.isSelected)) {
      setPendingGeneralModelId(null);
    }
  }, [generalChatModels, pendingGeneralModelId]);

  const selectedConnections = useMemo(
    () => connections.filter((connection) => selectedConnectionIds.includes(connection.id)),
    [connections, selectedConnectionIds],
  );

  const activeKnowledgeLabels = useMemo(() => {
    if (knowledgeMode === 'all') {
      return [t('chat.knowledgeAttachedAll')];
    }
    if (knowledgeMode === 'selected') {
      return selectedConnections.map((connection) => connection.name);
    }
    return [];
  }, [knowledgeMode, selectedConnections, t]);

  const openKnowledgeScreen = () => {
    Keyboard.dismiss();
    setShowComposerMenu(false);
    setShowKnowledgePicker(false);
    setShowHistoryDrawer(false);
    router.push('/knowledge');
  };

  const openKnowledgePicker = () => {
    setShowComposerMenu(false);
    if (!hasConnections) {
      openKnowledgeScreen();
      return;
    }
    setShowKnowledgePicker(true);
  };

  const openSkillPicker = () => {
    setShowComposerMenu(false);
    setShowSkillPicker(true);
  };

  const clearKnowledge = () => {
    setKnowledgeMode('none');
    setSelectedConnectionIds([]);
    setShowComposerMenu(false);
    setShowKnowledgePicker(false);
  };

  const selectAllKnowledge = () => {
    setKnowledgeMode('all');
    setSelectedConnectionIds([]);
  };

  const toggleKnowledgeConnection = (connectionId: string) => {
    setKnowledgeMode('selected');
    setSelectedConnectionIds((current) => {
      const next = current.includes(connectionId)
        ? current.filter((id) => id !== connectionId)
        : [...current, connectionId];
      if (next.length === 0) {
        setKnowledgeMode('none');
      }
      return next;
    });
  };

  const stopStreamingIfNeeded = () => {
    streamCleanupRef.current?.();
    streamCleanupRef.current = null;
    setStreaming(false);
  };

  const openModelPicker = () => {
    const defaultWidth = Math.min(284, windowWidth - 24);
    const fallbackPosition = {
      top: insets.top + 72,
      left: Math.max(12, windowWidth - defaultWidth - 12),
      width: defaultWidth,
    };
    const anchor = modelAnchorRef.current;

    if (!anchor) {
      setModelMenuPosition(fallbackPosition);
      setShowModelPicker(true);
      return;
    }

    requestAnimationFrame(() => {
      anchor.measureInWindow((x, y, width, height) => {
        const measuredWidth = width > 0 ? width : defaultWidth;
        const measuredHeight = height > 0 ? height : 0;
        const measuredTop = y > 0 ? y + measuredHeight + 12 : fallbackPosition.top;
        const measuredLeft = x > 0 ? x : fallbackPosition.left;
        const menuWidth = Math.min(Math.max(measuredWidth + 28, 284), windowWidth - 24);
        const left = Math.min(Math.max(12, measuredLeft), windowWidth - menuWidth - 12);
        setModelMenuPosition({
          top: measuredTop,
          left,
          width: menuWidth,
        });
        setShowModelPicker(true);
      });
    });
  };

  const closeSessionActions = () => {
    setSessionActionsMenuPosition(null);
    setSessionActionTarget(null);
  };

  const startNewChat = () => {
    stopStreamingIfNeeded();
    setShowHistoryDrawer(false);
    closeSessionActions();
    setActiveSessionId(null);
    setMessages([]);
    setInput('');
  };

  const openComposeRun = () => {
    Keyboard.dismiss();
    stopStreamingIfNeeded();
    setShowHistoryDrawer(false);
    closeSessionActions();
    router.push('/(modals)/compose');
  };

  const openRunHistory = () => {
    Keyboard.dismiss();
    setShowHistoryDrawer(false);
    closeSessionActions();
    router.push('/(modals)/run-history');
  };

  const openSession = (sessionId: string) => {
    if (sessionId === activeSessionId) {
      setShowHistoryDrawer(false);
      return;
    }
    stopStreamingIfNeeded();
    setShowHistoryDrawer(false);
    closeSessionActions();
    setActiveSessionId(sessionId);
    setMessages([]);
  };

  const openSessionActions = (session: ChatSession, anchorRect: MenuAnchorRect | null) => {
    const menuWidth = 232;
    const menuHeight = 192;
    const menuGap = 8;
    const fallbackTop = 96;
    const fallbackLeft = 16;

    const top = anchorRect
      ? Math.max(
          16,
          Math.min(anchorRect.top + anchorRect.height + menuGap, windowHeight - menuHeight - 16),
        )
      : fallbackTop;
    const left = anchorRect
      ? Math.max(
          16,
          Math.min(anchorRect.left, windowWidth - menuWidth - 16),
        )
      : fallbackLeft;

    setSessionActionTarget(session);
    setRenameSessionTitle(session.title);
    setSessionActionsMenuPosition({
      top,
      left,
      width: menuWidth,
    });
  };

  const renameSession = async () => {
    if (!sessionActionTarget || sessionActionBusy) {
      return;
    }
    const nextTitle = renameSessionTitle.trim();
    if (!nextTitle) {
      return;
    }
    setSessionActionBusy(true);
    try {
      await api.updateChatSession(accessToken, sessionActionTarget.id, {title: nextTitle});
      await queryClient.invalidateQueries({queryKey: ['chat-sessions']});
      setShowRenameSession(false);
      setSessionActionTarget(null);
    } finally {
      setSessionActionBusy(false);
    }
  };

  const deleteSession = async (target: ChatSession) => {
    if (sessionActionBusy) {
      return;
    }
    setSessionActionBusy(true);
    try {
      await api.deleteChatSession(accessToken, target.id);
      const remainingSessions = (sessionsQuery.data?.items ?? []).filter(
        (session) => session.id !== target.id,
      );
      if (target.id === activeSessionId) {
        setActiveSessionId(remainingSessions[0]?.id ?? null);
        if (remainingSessions.length === 0) {
          setMessages([]);
        }
      }
      await Promise.all([
        queryClient.invalidateQueries({queryKey: ['chat-sessions']}),
        queryClient.invalidateQueries({queryKey: ['chat-messages', target.id]}),
      ]);
      setShowRenameSession(false);
      closeSessionActions();
    } finally {
      setSessionActionBusy(false);
    }
  };

  const confirmDeleteSession = () => {
    if (!sessionActionTarget || sessionActionBusy) {
      return;
    }

    setSessionActionsMenuPosition(null);
    setShowDeleteSessionConfirm(true);
  };

  const formatSessionTime = (session: ChatSession) => {
    const raw = session.lastMessageAt ?? session.updatedAt ?? session.createdAt;
    if (!raw) {
      return '';
    }
    const date = new Date(raw);
    const month = `${date.getMonth() + 1}`;
    const day = `${date.getDate()}`;
    const hours = `${date.getHours()}`.padStart(2, '0');
    const minutes = `${date.getMinutes()}`.padStart(2, '0');
    return `${month}/${day} ${hours}:${minutes}`;
  };

  const toggleSessionPin = async () => {
    if (!sessionActionTarget || sessionActionBusy) {
      return;
    }
    setSessionActionBusy(true);
    try {
      await api.updateChatSession(accessToken, sessionActionTarget.id, {
        pinned: !sessionActionTarget.pinned,
      });
      await queryClient.invalidateQueries({queryKey: ['chat-sessions']});
      setSessionActionTarget(null);
      setSessionActionsMenuPosition(null);
    } finally {
      setSessionActionBusy(false);
    }
  };

  const sendMessage = () => {
    const message = input.trim();
    if (!message || streaming) {
      return;
    }

    const userMessage: ChatMessage = {
      id: `${Date.now()}-user`,
      role: 'user',
      content: message,
      state: 'done',
    };
    const assistantId = `${Date.now()}-assistant`;
    const assistantMessage: ChatMessage = {
      id: assistantId,
      role: 'assistant',
      content: '',
      sources: [],
      state: 'streaming',
    };

    setMessages((current) => [...current, userMessage, assistantMessage]);
    setInput('');
    setStreaming(true);
    streamCleanupRef.current?.();
    let liveSessionId = activeSessionId;

    streamCleanupRef.current = subscribeChatStream(
      accessToken,
      {
        message,
        skill: selectedSkill.mode,
        skillId: selectedSkill.id !== DEFAULT_SKILL_ID ? selectedSkill.id : undefined,
        connectionIds: knowledgeMode === 'selected' ? selectedConnectionIds : [],
        useKnowledge: knowledgeMode !== 'none',
        enableSearch,
        temperature: CHAT_REQUEST_TEMPERATURE,
        skillPrompt: selectedSkill.prompt,
        sessionId: activeSessionId ?? undefined,
      },
      (event) => {
        if (event.type === 'session') {
          if (event.session_id) {
            liveSessionId = event.session_id;
            setActiveSessionId(event.session_id);
            queryClient.invalidateQueries({queryKey: ['chat-sessions']}).catch(() => {});
          }
          return;
        }

        if (event.type === 'retrieval') {
          return;
        }

        if (event.type === 'chunk') {
          setMessages((current) =>
            current.map((item) =>
              item.id === assistantId
                ? {...item, content: `${item.content}${event.content ?? ''}`}
                : item,
            ),
          );
          return;
        }

        if (event.type === 'done') {
          setStreaming(false);
          setMessages((current) =>
            current.map((item) =>
              item.id === assistantId
                ? {
                    ...item,
                    content: event.content ?? item.content,
                    sources: event.sources ?? item.sources,
                    usage: event.usage ?? item.usage,
                    state: 'done',
                  }
                : item,
            ),
          );
          streamCleanupRef.current?.();
          streamCleanupRef.current = null;
          queryClient
            .invalidateQueries({queryKey: ['chat-messages', liveSessionId]})
            .catch(() => {});
          return;
        }

        if (event.type === 'error') {
          setStreaming(false);
          setMessages((current) =>
            current.map((item) =>
              item.id === assistantId
                ? {
                    ...item,
                    content: event.error ?? t('chat.error'),
                    state: 'error',
                  }
                : item,
            ),
          );
          streamCleanupRef.current?.();
          streamCleanupRef.current = null;
        }
      },
    );
  };

  const composerBottomPadding =
    Math.max(insets.bottom, 16) +
    (Platform.OS === 'android' ? keyboardInset : 0) +
    (Platform.OS === 'android' && keyboardInset > 0 ? 12 : 0);
  const keyboardPageOffset =
    Platform.OS === 'android' && keyboardInset > 0 ? keyboardInset + 20 : 0;

  const openHistoryDrawer = () => {
    Keyboard.dismiss();
    setShowHistoryDrawer(true);
  };

  const openProfileFromHistory = async () => {
    if (profileNavigationPendingRef.current) {
      return;
    }
    Keyboard.dismiss();
    profileNavigationPendingRef.current = true;
    setShowHistoryDrawer(false);

    try {
      const finished = await runProfileExitAnimation();
      if (!finished) {
        return;
      }
      router.push('/profile');
    } finally {
      profileNavigationPendingRef.current = false;
      resetProfileExitAnimation();
    }
  };

  return (
    <View className="flex-1 pb-10" style={{backgroundColor: colors.background}}>
      <View className="flex-1" style={{backgroundColor: colors.background}}>
        <Animated.View
          pointerEvents={showHistoryDrawer ? 'auto' : 'none'}
          style={{
            position: 'absolute',
            top: 0,
            bottom: 0,
            left: 0,
            width: drawerWidth,
            transform: [{translateX: drawerTranslateX}],
            zIndex: 1,
          }}>
          <HistoryDrawer
            colors={colors}
            width={drawerWidth}
            title={t('chat.sessionPickerTitle')}
            newChatLabel={t('chat.newChat')}
            startRunLabel={t('compose.startRun')}
            runHistoryLabel={t('runs.history')}
            emptyLabel={t('chat.sessionEmpty')}
            recentLabel={t('chat.historyRecent')}
            userName={userName}
            activeSessionId={activeSessionId}
            highlightedSessionId={sessionActionTarget?.id ?? null}
            sessions={sessions}
            onClose={() => {
              setShowHistoryDrawer(false);
              setSessionActionTarget(null);
              setSessionActionsMenuPosition(null);
            }}
            onNewChat={startNewChat}
            onStartRun={openComposeRun}
            onOpenRunHistory={openRunHistory}
            onOpenProfile={openProfileFromHistory}
            onOpenSession={openSession}
            onOpenSessionActions={openSessionActions}
            formatSessionTime={formatSessionTime}
          />
        </Animated.View>

        <Animated.View
          className="flex-1"
          style={{
            opacity: contentExitOpacity,
            transform: [
              {translateX: contentTranslateX},
              {translateY: contentExitTranslateY},
              {scale: contentExitScale},
            ],
            backgroundColor: colors.background,
            shadowColor: colors.shadow,
            shadowOffset: {width: -8, height: 0},
            shadowOpacity: showHistoryDrawer ? 0.12 : 0,
            shadowRadius: 20,
            elevation: showHistoryDrawer ? 18 : 0,
            zIndex: 2,
          }}>
          <SafeAreaView
            className="flex-1"
            style={{backgroundColor: colors.background}}
            edges={['top', 'bottom', 'left', 'right']}>
            <KeyboardAvoidingView
              className="flex-1"
              behavior={Platform.OS === 'ios' ? 'padding' : undefined}
              keyboardVerticalOffset={0}>
              <View className="flex-1">
                <View className="px-5 pb-4 pt-4">
                  <View className="flex-row items-center justify-between">
                    <Pressable
                      className="h-10 w-10 items-center justify-center rounded-full"
                      style={{backgroundColor: colors.surface}}
                      onPress={openHistoryDrawer}>
                      <Ionicons name="menu-outline" size={22} color={colors.textPrimary} />
                    </Pressable>
                    <View ref={modelAnchorRef} collapsable={false} className="mx-3 flex-1">
                      <View className="px-2 py-2">
                        <Pressable
                          className="max-w-full flex-row items-center justify-start gap-1.5 self-start"
                          onPress={openModelPicker}>
                          <Text
                            className="shrink text-left text-base font-semibold"
                            numberOfLines={1}
                            style={{
                              lineHeight: 22,
                              color:
                                selectedModel.available === false
                                  ? colors.textMuted
                                  : colors.textPrimary,
                            }}>
                            {selectedModel.title}
                          </Text>
                          <View className="flex-row items-center gap-1 self-center shrink-0">
                            <View className="h-5 items-center justify-center">
                              <Ionicons name="chevron-down" size={20} color={colors.textMuted} />
                            </View>
                            <Text
                              className="self-center text-[12px]"
                              style={{color: colors.textSecondary, lineHeight: 14}}>
                              {selectedModelTemperatureLabel}
                            </Text>
                          </View>
                        </Pressable>
                      </View>
                    </View>
                    <View className="flex-row items-center gap-2">
                      <Pressable
                        className="h-10 w-10 items-center justify-center rounded-full"
                        style={{backgroundColor: colors.surface}}
                        onPress={openKnowledgeScreen}>
                        <Ionicons name="book-outline" size={19} color={colors.textPrimary} />
                      </Pressable>
                      <Pressable
                        className="h-10 w-10 items-center justify-center rounded-full"
                        style={{backgroundColor: colors.surface}}
                        onPress={() => setShowSkillPicker(true)}>
                        <Ionicons name="construct-outline" size={19} color={colors.textPrimary} />
                      </Pressable>
                    </View>
                  </View>
                </View>

                <View
                  className="flex-1"
                  style={{
                    transform: [{translateY: -keyboardPageOffset}],
                  }}>
                  <ChatMessages
                    colors={colors}
                    messages={messages}
                    scrollRef={scrollRef}
                    assistantTitle="Knowvia"
                    assistantBadgeLabel={selectedModel.title}
                    greetingBody={t('chat.greetingBody')}
                    sendingLabel={t('chat.sending')}
                    sourcesLabel={t('chat.sources')}
                    contentBottomPadding={Math.max(24, composerHeight)}
                  />

                  <ChatComposer
                    colors={colors}
                    input={input}
                    streaming={streaming}
                    placeholder={t('chat.inputPlaceholder')}
                    composerBottomPadding={composerBottomPadding}
                    selectedSkillTitle={selectedSkill.title}
                    showSkillChip={selectedSkill.id !== DEFAULT_SKILL_ID}
                    activeKnowledgeLabels={activeKnowledgeLabels}
                    enableSearch={enableSearch}
                    searchLabel={t('chat.webSearch')}
                    searchStateLabel={enableSearch ? t('chat.webSearchEnabled') : t('chat.webSearchDisabled')}
                    onInputChange={setInput}
                    onSend={sendMessage}
                    onOpenComposerMenu={() => setShowComposerMenu(true)}
                    onOpenSkillPicker={() => setShowSkillPicker(true)}
                    onOpenKnowledgePicker={() => setShowKnowledgePicker(true)}
                    onEnableSearchChange={setEnableSearch}
                    onHeightChange={setComposerHeight}
                  />
                </View>
              </View>
            </KeyboardAvoidingView>

            <Animated.View
              pointerEvents={showHistoryDrawer ? 'auto' : 'none'}
              style={{
                position: 'absolute',
                top: 0,
                right: 0,
                bottom: 0,
                left: 0,
                backgroundColor: colors.overlay,
                opacity: contentOverlayOpacity,
              }}>
              <Pressable
                className="flex-1"
                onPress={() => {
                  setShowHistoryDrawer(false);
                  setSessionActionTarget(null);
                  setSessionActionsMenuPosition(null);
                }}
              />
            </Animated.View>
          </SafeAreaView>
        </Animated.View>
      </View>

      <ModelMenu
        colors={colors}
        visible={showModelPicker}
        position={modelMenuPosition}
        models={availableModels}
        selectedModelId={selectedModel.id}
        onClose={() => setShowModelPicker(false)}
        onSelect={(modelId) => {
          const targetModel = availableModels.find((model) => model.id === modelId);
          if (targetModel?.available === false) {
            return;
          }
          setPendingGeneralModelId(modelId);
          setShowModelPicker(false);
          void api
            .selectChatModel(accessToken, modelId)
            .then(async () => {
              await queryClient.invalidateQueries({queryKey: ['chat-models']});
            })
            .catch((error: unknown) => {
              if (modelId === 'default-general-model') {
                setPendingGeneralModelId(null);
                return;
              }
              setPendingGeneralModelId(null);
              showTatos({
                title: t('models.selectFailed'),
                body:
                  error instanceof Error && error.message.trim()
                    ? error.message
                    : t('models.selectFailed'),
              });
            });
        }}
      />

      <SessionActionsMenu
        colors={colors}
        visible={Boolean(sessionActionTarget && sessionActionsMenuPosition)}
        position={sessionActionsMenuPosition}
        renameLabel={t('chat.renameSession')}
        pinLabel={
          sessionActionTarget?.pinned
            ? t('chat.unpinSession')
            : t('chat.pinSession')
        }
        deleteLabel={t('chat.deleteSession')}
        pinned={Boolean(sessionActionTarget?.pinned)}
        onClose={closeSessionActions}
        onRename={() => {
          setSessionActionsMenuPosition(null);
          setShowRenameSession(true);
        }}
        onPin={() => {
          void toggleSessionPin();
        }}
        onDelete={confirmDeleteSession}
      />

      <ConfirmModal
        visible={showDeleteSessionConfirm}
        title={t('chat.deleteSessionConfirmTitle')}
        body={t('chat.deleteSessionConfirmBody', {name: sessionActionTarget?.title ?? ''})}
        cancelLabel={t('common.cancel')}
        confirmLabel={t('chat.deleteSession')}
        onCancel={() => {
          setShowDeleteSessionConfirm(false);
          setSessionActionTarget(null);
        }}
        onConfirm={() => {
          const target = sessionActionTarget;
          setShowDeleteSessionConfirm(false);
          setSessionActionTarget(null);
          if (target) {
            void deleteSession(target);
          }
        }}
      />

      <Modal
        transparent
        visible={showRenameSession}
        animationType="fade"
        statusBarTranslucent
        onRequestClose={() => {
          setShowRenameSession(false);
          setSessionActionTarget(null);
        }}>
        <View className="flex-1 items-center justify-center px-6">
          <Pressable
            className="absolute inset-0"
            style={{backgroundColor: colors.overlaySoft}}
            onPress={() => {
              setShowRenameSession(false);
              setSessionActionTarget(null);
            }}
          />

          <View
            className="w-full rounded-[24px] px-5 py-5"
            style={{
              maxWidth: Math.min(windowWidth - 32, 360),
              backgroundColor: colors.surface,
              shadowColor: colors.shadow,
              shadowOffset: {width: 0, height: 14},
              shadowOpacity: 0.12,
              shadowRadius: 24,
              elevation: 18,
            }}>
            <Text className="mb-4 text-lg font-semibold" style={{color: colors.textPrimary}}>
              {t('chat.renameSessionTitle')}
            </Text>

            <View
              className="rounded-[18px] px-4 py-3"
              style={{backgroundColor: colors.surfaceMuted}}>
              <TextInput
                value={renameSessionTitle}
                onChangeText={setRenameSessionTitle}
                placeholder={t('chat.renameSessionPlaceholder')}
                placeholderTextColor={colors.textTertiary}
                style={{
                  color: colors.textPrimary,
                  fontSize: fontSizes.md,
                  minHeight: 24,
                  paddingVertical: 0,
                }}
              />
            </View>

            <View className="mt-5 flex-row gap-3">
              <View className="flex-1">
                <PrimaryButton
                  label={t('chat.cancel')}
                  secondary
                  onPress={() => {
                    setShowRenameSession(false);
                    setSessionActionTarget(null);
                  }}
                />
              </View>
              <View className="flex-1">
                <PrimaryButton label={t('chat.renameSessionSave')} onPress={renameSession} />
              </View>
            </View>
          </View>
        </View>
      </Modal>

      <BottomSheet
        visible={showComposerMenu}
        onClose={() => setShowComposerMenu(false)}
        title={t('chat.menuTitle')}>
        <OptionRow
          colors={colors}
          icon="book-outline"
          title={t('chat.menuKnowledge')}
          description={t('chat.menuKnowledgeHint')}
          note={t('chat.knowledgeModelSummary', {
            name: selectedKnowledgeModel.name,
            temperature: `T=${formatTemperature(selectedKnowledgeModel.temperature)}`,
          })}
          onPress={openKnowledgePicker}
        />
        <OptionRow
          colors={colors}
          icon="construct-outline"
          title={t('chat.menuSkills')}
          description={t('chat.menuSkillsHint')}
          onPress={openSkillPicker}
        />
        <OptionRow
          colors={colors}
          icon="link-outline"
          title={t('chat.menuManageKnowledge')}
          description={t('chat.menuManageKnowledgeHint')}
          onPress={openKnowledgeScreen}
        />
        {knowledgeMode !== 'none' ? (
          <Pressable
            className="mt-2 items-center rounded-[18px] px-4 py-4"
            style={{backgroundColor: colors.surfaceMuted}}
            onPress={clearKnowledge}>
            <Text className="font-semibold" style={{color: colors.textPrimary}}>
              {t('chat.menuRemoveKnowledge')}
            </Text>
          </Pressable>
        ) : null}
      </BottomSheet>

      <SkillPickerSheet
        colors={colors}
        visible={showSkillPicker}
        title={t('chat.skillPickerTitle')}
        manageLabel={t('chat.manageSkills')}
        skills={availableSkills}
        selectedSkillId={selectedSkill.id}
        onClose={() => setShowSkillPicker(false)}
        onManage={() => {
          setShowSkillPicker(false);
          router.push('/skills');
        }}
        onSelectSkill={(skillId) => {
          setSelectedSkillId(skillId);
          setShowSkillPicker(false);
        }}
      />

      <KnowledgePickerSheet
        colors={colors}
        visible={showKnowledgePicker}
        title={t('chat.knowledgePickerTitle')}
        doneLabel={t('chat.done')}
        detachedLabel={t('chat.knowledgeDetached')}
        allLabel={t('chat.knowledgeAll')}
        knowledgeMode={knowledgeMode}
        connections={connections}
        selectedConnectionIds={selectedConnectionIds}
        onClose={() => setShowKnowledgePicker(false)}
        onClearKnowledge={clearKnowledge}
        onSelectAllKnowledge={selectAllKnowledge}
        onToggleConnection={toggleKnowledgeConnection}
      />

      {tatosNode}
    </View>
  );
}
