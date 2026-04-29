import {useQuery} from '@tanstack/react-query';
import {useRouter} from 'expo-router';
import {Pressable, Text, View} from 'react-native';

import {PrimaryButton} from '@/components/PrimaryButton';
import {Screen} from '@/components/Screen';
import {StatusBadge} from '@/components/StatusBadge';
import {useI18n} from '@/i18n/useI18n';
import {api} from '@/lib/api';
import {useAuthStore} from '@/store/auth';
import type {Run} from '@/types/api';
import {useAppTheme} from '@/theme/useAppTheme';

function formatRunTime(raw: string) {
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return raw;
  }
  const month = `${date.getMonth() + 1}`.padStart(2, '0');
  const day = `${date.getDate()}`.padStart(2, '0');
  const hours = `${date.getHours()}`.padStart(2, '0');
  const minutes = `${date.getMinutes()}`.padStart(2, '0');
  return `${month}/${day} ${hours}:${minutes}`;
}

export default function RunHistoryScreen() {
  const router = useRouter();
  const accessToken = useAuthStore((state) => state.accessToken)!;
  const {colors} = useAppTheme();
  const {t, modeLabel} = useI18n();

  const runsQuery = useQuery({
    queryKey: ['runs'],
    queryFn: () => api.listRuns(accessToken),
  });

  const runs = [...(runsQuery.data?.items ?? [])].sort((left: Run, right: Run) => {
    const leftDate = left.updatedAt || left.createdAt;
    const rightDate = right.updatedAt || right.createdAt;
    return new Date(rightDate).getTime() - new Date(leftDate).getTime();
  });

  return (
    <Screen scroll>
      <View className="gap-5">
        <View className="flex-row items-center justify-between">
          <View className="flex-1 pr-4">
            <Text className="text-3xl font-bold" style={{color: colors.textPrimary}}>
              {t('runs.history')}
            </Text>
            <Text className="mt-2 text-base leading-7" style={{color: colors.textSecondary}}>
              {t('runs.historyDescription')}
            </Text>
          </View>
          {/* <Pressable
            className="h-10 w-10 items-center justify-center rounded-full"
            style={{backgroundColor: colors.surface}}
            onPress={() => router.back()}>
            <Text className="text-lg font-semibold" style={{color: colors.textPrimary}}>
              ←
            </Text>
          </Pressable> */}
        </View>

        <View className="gap-3">
          <PrimaryButton label={t('compose.startRun')} onPress={() => router.replace('/(modals)/compose')} />
        </View>

        {runsQuery.isLoading ? (
          <View className="rounded-[20px] p-5" style={{backgroundColor: colors.surface}}>
            <Text className="text-base" style={{color: colors.textSecondary}}>
              {t('runs.loading')}
            </Text>
          </View>
        ) : runsQuery.error ? (
          <View className="rounded-[20px] p-5" style={{backgroundColor: colors.surface}}>
            <Text className="text-base" style={{color: '#dc2626'}}>
              {runsQuery.error instanceof Error ? runsQuery.error.message : t('runs.loadFailed')}
            </Text>
          </View>
        ) : runs.length === 0 ? (
          <View className="gap-3 rounded-[20px] p-5" style={{backgroundColor: colors.surface}}>
            <Text className="text-xl font-semibold" style={{color: colors.textPrimary}}>
              {t('runs.emptyTitle')}
            </Text>
            <Text className="text-base leading-7" style={{color: colors.textSecondary}}>
              {t('runs.emptyDescription')}
            </Text>
          </View>
        ) : (
          <View className="gap-3">
            {runs.map((run) => (
              <Pressable
                key={run.id}
                className="gap-3 rounded-[24px] p-5"
                style={{backgroundColor: colors.surface}}
                onPress={() => router.push(`/runs/${run.id}`)}>
                <View className="flex-row items-start justify-between gap-3">
                  <View className="flex-1 gap-2">
                    <Text className="text-lg font-semibold" style={{color: colors.textPrimary}}>
                      {run.title}
                    </Text>
                    <Text className="text-sm" style={{color: colors.textSecondary}}>
                      {run.goal}
                    </Text>
                  </View>
                  <StatusBadge status={run.status} />
                </View>

                <View className="flex-row flex-wrap items-center gap-3">
                  <View
                    className="rounded-full px-3 py-1"
                    style={{backgroundColor: colors.surfaceMuted}}>
                    <Text className="text-xs font-semibold" style={{color: colors.textMuted}}>
                      {modeLabel(run.effectiveMode)}
                    </Text>
                  </View>
                  <Text className="text-xs" style={{color: colors.textTertiary}}>
                    {t('runs.updatedAt', {time: formatRunTime(run.updatedAt || run.createdAt)})}
                  </Text>
                </View>
              </Pressable>
            ))}
          </View>
        )}
      </View>
    </Screen>
  );
}
