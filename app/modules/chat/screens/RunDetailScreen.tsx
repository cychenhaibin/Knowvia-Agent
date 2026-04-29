import {useQuery, useQueryClient} from '@tanstack/react-query';
import Markdown from 'react-native-markdown-display';
import {useLocalSearchParams} from 'expo-router';
import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {ScrollView, Text, useWindowDimensions, View} from 'react-native';

import {GroundingTraceSection} from '@/modules/chat/components/GroundingTraceSection';
import {RunSourcesSection} from '@/modules/chat/components/RunSourcesSection';
import {Screen} from '@/components/Screen';
import {StatusBadge} from '@/components/StatusBadge';
import {TimelineStep} from '@/components/TimelineStep';
import {useI18n} from '@/i18n/useI18n';
import {api, subscribeRunEvents} from '@/lib/api';
import {useAuthStore} from '@/store/auth';
import {createMarkdownRules, createMarkdownStyles} from '@/theme/markdown';
import {useAppTheme} from '@/theme/useAppTheme';
import type {RunArtifact, RunEvent} from '@/types/api';

import {normalizeMarkdownLines, parseGroundedSourceRefs, sourceIdentity} from '../utils/runGrounding';

type ArtifactFocus = RunArtifact['kind'] | null;

export default function RunDetailScreen() {
  const queryClient = useQueryClient();
  const {id} = useLocalSearchParams<{id: string}>();
  const accessToken = useAuthStore((state) => state.accessToken)!;
  const [liveEvents, setLiveEvents] = useState<RunEvent[]>([]);
  const [focusedArtifactKind, setFocusedArtifactKind] = useState<ArtifactFocus>(null);
  const scrollViewRef = useRef<ScrollView | null>(null);
  const previousFocusRef = useRef<ArtifactFocus>(null);
  const groundingSectionYRef = useRef(0);
  const sourcePositionsRef = useRef<Record<string, number>>({});
  const pendingScrollTargetRef = useRef<string | null>(null);
  const [sourceLayoutVersion, setSourceLayoutVersion] = useState(0);
  const {colors} = useAppTheme();
  const {width: windowWidth} = useWindowDimensions();
  const {t, modeLabel} = useI18n();

  const runQuery = useQuery({
    queryKey: ['run', id],
    queryFn: () => api.getRun(accessToken, id!),
    enabled: !!id,
  });

  useEffect(() => {
    if (!id || !accessToken) {
      return;
    }
    return subscribeRunEvents(id, accessToken, (event) => {
      setLiveEvents((current) => [event, ...current].slice(0, 10));
      queryClient.invalidateQueries({queryKey: ['run', id]});
      queryClient.invalidateQueries({queryKey: ['runs']});
    });
  }, [accessToken, id, queryClient]);

  const report = useMemo(
    () => runQuery.data?.artifacts.find((artifact) => artifact.kind === 'report')?.contentMarkdown ?? '',
    [runQuery.data?.artifacts],
  );
  const reportOutline = useMemo(
    () => runQuery.data?.artifacts.find((artifact) => artifact.kind === 'report_outline')?.contentMarkdown ?? '',
    [runQuery.data?.artifacts],
  );
  const reportDraft = useMemo(
    () => runQuery.data?.artifacts.find((artifact) => artifact.kind === 'report_draft')?.contentMarkdown ?? '',
    [runQuery.data?.artifacts],
  );
  const reportGrounding = useMemo(
    () => runQuery.data?.artifacts.find((artifact) => artifact.kind === 'report_grounding')?.contentMarkdown ?? '',
    [runQuery.data?.artifacts],
  );
  const finalAnswer = useMemo(
    () => runQuery.data?.artifacts.find((artifact) => artifact.kind === 'final_answer')?.contentMarkdown ?? '',
    [runQuery.data?.artifacts],
  );
  const groundedSourceRefs = useMemo(() => parseGroundedSourceRefs(reportGrounding), [reportGrounding]);
  const [selectedGroundedSourceKey, setSelectedGroundedSourceKey] = useState<string | null>(null);
  const [showGroundedSourcesOnly, setShowGroundedSourcesOnly] = useState(false);
  const [groundedSourceFilterOrigin, setGroundedSourceFilterOrigin] = useState<'auto' | 'manual' | 'none'>('none');
  const revisionSummary = useMemo(() => {
    if (!reportDraft.trim() || !report.trim()) {
      return null;
    }
    const draftLines = normalizeMarkdownLines(reportDraft);
    const finalLines = normalizeMarkdownLines(report);
    const draftSet = new Set(draftLines);
    const finalSet = new Set(finalLines);
    const addedLines = finalLines.filter((line) => !draftSet.has(line));
    const removedLines = draftLines.filter((line) => !finalSet.has(line));

    if (addedLines.length === 0 && removedLines.length === 0) {
      return {
        title: t('detail.revisionSummary'),
        body: t('detail.revisionSummaryNoChanges'),
      };
    }

    return {
      title: t('detail.revisionSummary'),
      body: t('detail.revisionSummaryChanged', {
        added: addedLines.length,
        removed: removedLines.length,
      }),
      examples: [...addedLines.slice(0, 2), ...removedLines.slice(0, 2)],
    };
  }, [report, reportDraft, t]);
  const stagedArtifacts = useMemo<
    Array<{
      kind: RunArtifact['kind'];
      title: string;
      description: string;
      badge: string;
      content: string;
    }>
  >(
    () => [
      {
        kind: 'report_outline',
        title: t('detail.reportOutline'),
        description: t('detail.reportOutlineDescription'),
        badge: t('detail.stageOne'),
        content: reportOutline,
      },
      {
        kind: 'report_draft',
        title: t('detail.reportDraft'),
        description: t('detail.reportDraftDescription'),
        badge: t('detail.stageTwo'),
        content: reportDraft,
      },
      {
        kind: 'report',
        title: t('detail.finalReport'),
        description: t('detail.finalReportDescription'),
        badge: t('detail.stageThree'),
        content: report,
      },
    ],
    [report, reportDraft, reportOutline, t],
  );
  const artifactFocusByStep = useMemo<Record<string, ArtifactFocus>>(
    () => ({
      planning: 'report_outline',
      yuque_search: 'report_grounding',
      web_search: 'report_grounding',
      web_page_extract: 'report_grounding',
      evidence_merge: 'report_draft',
      report_writer: 'report',
      finalize: 'final_answer',
    }),
    [],
  );

  const markdownTableViewportWidth = Math.max(windowWidth - 88, 0);
  const markdownTableCellMinWidth = Math.max(Math.round((windowWidth - 88) * 0.65), 180);
  const markdownStyles = useMemo(
    () => createMarkdownStyles(colors, markdownTableCellMinWidth),
    [colors, markdownTableCellMinWidth],
  );
  const markdownRules = useMemo(
    () => createMarkdownRules(markdownTableCellMinWidth, markdownTableViewportWidth),
    [markdownTableCellMinWidth, markdownTableViewportWidth],
  );
  const sources = runQuery.data?.sources ?? [];
  const groundedSourceKeys = useMemo(
    () => new Set(groundedSourceRefs.map((item) => item.key)),
    [groundedSourceRefs],
  );
  const orderedSources = useMemo(() => {
    const items = [...sources];
    items.sort((left, right) => {
      const leftKey = sourceIdentity(left);
      const rightKey = sourceIdentity(right);
      const leftSelected = selectedGroundedSourceKey != null && leftKey === selectedGroundedSourceKey ? 1 : 0;
      const rightSelected = selectedGroundedSourceKey != null && rightKey === selectedGroundedSourceKey ? 1 : 0;
      if (leftSelected !== rightSelected) {
        return rightSelected - leftSelected;
      }
      if (focusedArtifactKind === 'report_grounding') {
        const leftGrounded = groundedSourceKeys.has(leftKey) ? 1 : 0;
        const rightGrounded = groundedSourceKeys.has(rightKey) ? 1 : 0;
        if (leftGrounded !== rightGrounded) {
          return rightGrounded - leftGrounded;
        }
      }
      return right.score - left.score;
    });
    return items;
  }, [focusedArtifactKind, groundedSourceKeys, selectedGroundedSourceKey, sources]);
  const filteredSources = useMemo(() => {
    if (!showGroundedSourcesOnly) {
      return orderedSources;
    }
    return orderedSources.filter((source) => groundedSourceKeys.has(sourceIdentity(source)));
  }, [groundedSourceKeys, orderedSources, showGroundedSourcesOnly]);
  const groundedSourceCount = useMemo(
    () => sources.filter((source) => groundedSourceKeys.has(sourceIdentity(source))).length,
    [groundedSourceKeys, sources],
  );
  const selectedGroundedSourcePreview = useMemo(() => {
    if (!selectedGroundedSourceKey) {
      return null;
    }
    return sources.find((source) => sourceIdentity(source) === selectedGroundedSourceKey) ?? null;
  }, [selectedGroundedSourceKey, sources]);
  const selectedGroundedSourceIndex = useMemo(
    () => groundedSourceRefs.findIndex((item) => item.key === selectedGroundedSourceKey),
    [groundedSourceRefs, selectedGroundedSourceKey],
  );

  const flushPendingSourceScroll = useCallback(() => {
    const key = pendingScrollTargetRef.current;
    if (!key) {
      return;
    }
    const y = sourcePositionsRef.current[key];
    if (typeof y !== 'number') {
      return;
    }
    scrollViewRef.current?.scrollTo({
      y: Math.max(y - 24, 0),
      animated: true,
    });
    pendingScrollTargetRef.current = null;
  }, []);

  const scrollToGroundingSection = useCallback(() => {
    scrollViewRef.current?.scrollTo({
      y: Math.max(groundingSectionYRef.current - 24, 0),
      animated: true,
    });
  }, []);

  useEffect(() => {
    if (!selectedGroundedSourceKey || pendingScrollTargetRef.current !== selectedGroundedSourceKey) {
      return;
    }
    const timer = setTimeout(() => {
      flushPendingSourceScroll();
    }, 0);
    return () => clearTimeout(timer);
  }, [flushPendingSourceScroll, orderedSources, selectedGroundedSourceKey, sourceLayoutVersion]);

  useEffect(() => {
    if (!selectedGroundedSourceKey) {
      return;
    }
    if (groundedSourceKeys.has(selectedGroundedSourceKey)) {
      return;
    }
    setSelectedGroundedSourceKey(null);
    pendingScrollTargetRef.current = null;
  }, [groundedSourceKeys, selectedGroundedSourceKey]);

  useEffect(() => {
    if (groundedSourceKeys.size > 0) {
      return;
    }
    setShowGroundedSourcesOnly(false);
    setGroundedSourceFilterOrigin('none');
  }, [groundedSourceKeys]);

  useEffect(() => {
    const previousFocus = previousFocusRef.current;
    if (focusedArtifactKind === 'report_grounding' && previousFocus !== 'report_grounding' && groundedSourceKeys.size > 0) {
      setShowGroundedSourcesOnly(true);
      setGroundedSourceFilterOrigin('auto');
    }
    if (focusedArtifactKind !== 'report_grounding' && previousFocus === 'report_grounding') {
      setShowGroundedSourcesOnly(false);
      setGroundedSourceFilterOrigin('none');
    }
    previousFocusRef.current = focusedArtifactKind;
  }, [focusedArtifactKind, groundedSourceKeys]);

  const selectGroundedSourceFromTrace = useCallback(
    (key: string) => {
      const nextKey = selectedGroundedSourceKey === key ? null : key;
      pendingScrollTargetRef.current = nextKey;
      setSelectedGroundedSourceKey(nextKey);
    },
    [selectedGroundedSourceKey],
  );

  const selectGroundedSourceFromSource = useCallback(
    (key: string) => {
      const nextKey = selectedGroundedSourceKey === key ? null : key;
      setSelectedGroundedSourceKey(nextKey);
      if (nextKey) {
        setFocusedArtifactKind('report_grounding');
        scrollToGroundingSection();
      }
    },
    [scrollToGroundingSection, selectedGroundedSourceKey],
  );
  const stepGroundedSourcePreview = useCallback(
    (offset: number) => {
      if (selectedGroundedSourceIndex < 0) {
        return;
      }
      const nextItem = groundedSourceRefs[selectedGroundedSourceIndex + offset];
      if (!nextItem) {
        return;
      }
      selectGroundedSourceFromTrace(nextItem.key);
    },
    [groundedSourceRefs, selectedGroundedSourceIndex, selectGroundedSourceFromTrace],
  );

  if (!runQuery.data) {
    return (
      <Screen>
        <View className="flex-1 items-center justify-center">
          <Text className="text-base" style={{color: colors.textSecondary}}>
            {t('detail.loading')}
          </Text>
        </View>
      </Screen>
    );
  }

  const {run, steps} = runQuery.data;

  return (
    <Screen scroll scrollViewRef={scrollViewRef}>
      <View className="gap-6">
        <View className="gap-3 rounded-[28px] p-5" style={{backgroundColor: colors.surface}}>
          <StatusBadge status={run.status} />
          <Text className="text-2xl font-bold" style={{color: colors.textPrimary}}>
            {run.title}
          </Text>
          <Text className="text-base leading-7" style={{color: colors.textSecondary}}>
            {run.goal}
          </Text>
          <Text className="text-xs uppercase tracking-[2px]" style={{color: colors.textTertiary}}>
            {t('detail.effectiveMode', {mode: modeLabel(run.effectiveMode)})}
          </Text>
        </View>

        <View className="gap-3">
          <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
            {t('detail.executionTimeline')}
          </Text>
          <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
            {t('detail.timelineArtifactHint')}
          </Text>
          {steps.map((step) => (
            <TimelineStep
              key={step.id}
              step={step}
              active={artifactFocusByStep[step.kind] === focusedArtifactKind}
              onPress={() => {
                const nextFocus = artifactFocusByStep[step.kind];
                if (nextFocus) {
                  setFocusedArtifactKind((current) => (current === nextFocus ? null : nextFocus));
                }
              }}
            />
          ))}
        </View>

        <View className="gap-3">
          <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
            {t('detail.finalAnswer')}
          </Text>
          <View
            className="gap-3 rounded-[28px] p-5"
            style={{
              backgroundColor: colors.surface,
              borderWidth: focusedArtifactKind === 'final_answer' ? 2 : 1,
              borderColor: focusedArtifactKind === 'final_answer' ? colors.brand : colors.border,
            }}>
            <View className="gap-2">
              <View
                className="self-start rounded-full px-3 py-1"
                style={{backgroundColor: colors.surfaceMuted, borderWidth: 1, borderColor: colors.border}}>
                <Text className="text-[11px] font-semibold uppercase tracking-[1px]" style={{color: colors.textPrimary}}>
                  {t('detail.finalAnswerBadge')}
                </Text>
              </View>
              <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
                {t('detail.finalAnswerDescription')}
              </Text>
            </View>
            {finalAnswer ? (
              <Markdown rules={markdownRules} style={markdownStyles}>
                {finalAnswer}
              </Markdown>
            ) : (
              <Text className="text-sm" style={{color: colors.textSecondary}}>
                {t('detail.waitingStageArtifact')}
              </Text>
            )}
          </View>
        </View>

        <GroundingTraceSection
          focused={focusedArtifactKind === 'report_grounding'}
          reportGrounding={reportGrounding}
          groundedSourceRefs={groundedSourceRefs}
          selectedGroundedSourceKey={selectedGroundedSourceKey}
          selectedGroundedSourcePreview={selectedGroundedSourcePreview}
          selectedGroundedSourceIndex={selectedGroundedSourceIndex}
          markdownRules={markdownRules}
          markdownStyles={markdownStyles}
          onSectionLayout={(y) => {
            groundingSectionYRef.current = y;
          }}
          onSelectGroundedSource={selectGroundedSourceFromTrace}
          onStepGroundedSourcePreview={stepGroundedSourcePreview}
        />

        <View className="gap-3">
          <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
            {t('detail.reportStages')}
          </Text>
          {revisionSummary ? (
            <View
              className="gap-2 rounded-[24px] p-4"
              style={{
                backgroundColor: colors.surface,
                borderWidth: 1,
                borderColor: colors.border,
              }}>
              <Text className="text-sm font-semibold" style={{color: colors.textPrimary}}>
                {revisionSummary.title}
              </Text>
              <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
                {revisionSummary.body}
              </Text>
              {'examples' in revisionSummary && revisionSummary.examples?.length ? (
                <View className="gap-1">
                  {revisionSummary.examples.map((line) => (
                    <Text key={line} className="text-xs leading-5" style={{color: colors.textTertiary}}>
                      {line}
                    </Text>
                  ))}
                </View>
              ) : null}
            </View>
          ) : null}
          {stagedArtifacts.map((artifact) => (
            <View
              key={artifact.kind}
              className="gap-3 rounded-[28px] p-5"
              style={{
                backgroundColor: colors.surface,
                borderWidth: focusedArtifactKind === artifact.kind ? 2 : 1,
                borderColor: focusedArtifactKind === artifact.kind ? colors.brand : colors.border,
              }}>
              <View className="gap-2">
                <View
                  className="self-start rounded-full px-3 py-1"
                  style={{backgroundColor: colors.surfaceMuted, borderWidth: 1, borderColor: colors.border}}>
                  <Text
                    className="text-[11px] font-semibold uppercase tracking-[1px]"
                    style={{color: colors.textPrimary}}>
                    {artifact.badge}
                  </Text>
                </View>
                <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
                  {artifact.title}
                </Text>
                <Text className="text-sm leading-6" style={{color: colors.textSecondary}}>
                  {artifact.description}
                </Text>
              </View>
              {artifact.content ? (
                <Markdown rules={markdownRules} style={markdownStyles}>
                  {artifact.content}
                </Markdown>
              ) : (
                <Text className="text-sm" style={{color: colors.textSecondary}}>
                  {artifact.kind === 'report' ? t('detail.waitingReport') : t('detail.waitingStageArtifact')}
                </Text>
              )}
            </View>
          ))}
        </View>

        <RunSourcesSection
          sources={sources}
          groundedSourceKeys={groundedSourceKeys}
          groundedSourceCount={groundedSourceCount}
          filteredSources={filteredSources}
          focusedArtifactKind={focusedArtifactKind}
          selectedGroundedSourceKey={selectedGroundedSourceKey}
          showGroundedSourcesOnly={showGroundedSourcesOnly}
          groundedSourceFilterOrigin={groundedSourceFilterOrigin}
          onShowAllSources={() => {
            setShowGroundedSourcesOnly(false);
            setGroundedSourceFilterOrigin('manual');
          }}
          onShowGroundedSources={() => {
            setShowGroundedSourcesOnly(true);
            setGroundedSourceFilterOrigin('manual');
          }}
          onSourceLayout={(key, y) => {
            if (sourcePositionsRef.current[key] === y) {
              return;
            }
            sourcePositionsRef.current[key] = y;
            setSourceLayoutVersion((current) => current + 1);
          }}
          onSelectGroundedSource={selectGroundedSourceFromSource}
        />

        <View className="gap-3">
          <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
            {t('detail.liveEvents')}
          </Text>
          <View className="rounded-[28px] p-5" style={{backgroundColor: colors.surface}}>
            {liveEvents.length ? (
              liveEvents.map((event) => (
                <View
                  key={`${event.type}-${event.timestamp}`}
                  className="py-3"
                  style={{
                    borderBottomWidth: 1,
                    borderBottomColor: colors.divider,
                  }}>
                  <Text className="text-sm font-semibold" style={{color: colors.textPrimary}}>
                    {event.type}
                  </Text>
                  <Text className="mt-1 text-xs" style={{color: colors.textTertiary}}>
                    {new Date(event.timestamp).toLocaleTimeString()}
                  </Text>
                </View>
              ))
            ) : (
              <Text className="text-sm" style={{color: colors.textSecondary}}>
                {t('detail.waitingEvents')}
              </Text>
            )}
          </View>
        </View>
      </View>
    </Screen>
  );
}
