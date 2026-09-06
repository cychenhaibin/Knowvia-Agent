import {Ionicons} from '@expo/vector-icons';
import {useQuery} from '@tanstack/react-query';
import {useRouter} from 'expo-router';
import {ActivityIndicator, Pressable, ScrollView, Text, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {PrimaryButton} from '@/components/PrimaryButton';
import {useI18n} from '@/i18n/useI18n';
import {api} from '@/lib/api';
import {useAuthStore} from '@/store/auth';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';
import type {FluxAModelGroup} from '@/types/api';

function errorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}

function FluxAModelGroupSection({group}: {group: FluxAModelGroup}) {
  const {colors} = useAppTheme();

  return (
    <View className="gap-3 rounded-[18px] p-4" style={{backgroundColor: colors.surface}}>
      <Text style={{fontSize: fontSizes.lg, fontWeight: '700', color: colors.textPrimary}}>
        {group.name}
      </Text>
      <View className="gap-2">
        {group.models.map((model) => (
          <View
            key={model.id}
            className="rounded-xl px-3 py-2.5"
            style={{backgroundColor: colors.surfaceMuted}}>
            <Text style={{fontSize: fontSizes.sm, color: colors.textPrimary}}>{model.name}</Text>
          </View>
        ))}
      </View>
    </View>
  );
}

export default function FluxAModelGroupsScreen() {
  const router = useRouter();
  const accessToken = useAuthStore((state) => state.accessToken);
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const modelGroupsQuery = useQuery({
    queryKey: ['fluxa-model-groups'],
    queryFn: () => api.listFluxAModelGroups(accessToken!),
    enabled: Boolean(accessToken),
  });
  const groups = modelGroupsQuery.data ?? [];

  return (
    <SafeAreaView
      className="flex-1"
      style={{backgroundColor: colors.background}}
      edges={['top', 'bottom', 'left', 'right']}>
      <View
        className="mb-2 flex-row items-center justify-between px-3 pt-3"
        style={{backgroundColor: colors.background}}>
        <Pressable
          className="h-11 w-11 items-center justify-center rounded-full"
          onPress={() => router.back()}>
          <Ionicons name="chevron-back" size={22} color={colors.textPrimary} />
        </Pressable>
        <Text style={{fontSize: fontSizes.lg, fontWeight: '500', color: colors.textPrimary}}>
          {t('fluxaModelGroups.title')}
        </Text>
        <View className="w-11" />
      </View>

      {modelGroupsQuery.isLoading && !modelGroupsQuery.data ? (
        <View className="flex-1 items-center justify-center gap-3 px-8">
          <ActivityIndicator size="large" color={colors.brand} />
          <Text style={{fontSize: fontSizes.sm, color: colors.textSecondary}}>
            {t('fluxaModelGroups.loading')}
          </Text>
        </View>
      ) : modelGroupsQuery.error ? (
        <View className="flex-1 items-center justify-center gap-4 px-8">
          <Text
            className="text-center"
            style={{fontSize: fontSizes.sm, lineHeight: 22, color: colors.textSecondary}}>
            {errorMessage(modelGroupsQuery.error, t('fluxaModelGroups.loadFailed'))}
          </Text>
          <View className="min-w-[140px]">
            <PrimaryButton
              label={t('fluxaModelGroups.retry')}
              onPress={() => {
                void modelGroupsQuery.refetch();
              }}
            />
          </View>
        </View>
      ) : (
        <ScrollView
          className="flex-1"
          contentContainerStyle={{paddingHorizontal: 20, paddingBottom: 28}}
          showsVerticalScrollIndicator={false}>
          <View className="gap-5">
            <Text style={{fontSize: fontSizes.sm, lineHeight: 22, color: colors.textSecondary}}>
              {t('fluxaModelGroups.description')}
            </Text>
            {groups.length === 0 ? (
              <Text style={{fontSize: fontSizes.sm, color: colors.textMuted}}>
                {t('fluxaModelGroups.empty')}
              </Text>
            ) : (
              groups.map((group) => <FluxAModelGroupSection key={group.name} group={group} />)
            )}
          </View>
        </ScrollView>
      )}
    </SafeAreaView>
  );
}
