import {Ionicons} from '@expo/vector-icons';
import {useQuery} from '@tanstack/react-query';
import {useRouter} from 'expo-router';
import {useState} from 'react';
import {ActivityIndicator, Pressable, ScrollView, Text, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {FluxAIcon} from '@/components/FluxAIcon';
import {PrimaryButton} from '@/components/PrimaryButton';
import {useI18n} from '@/i18n/useI18n';
import {api} from '@/lib/api';
import {useAuthStore} from '@/store/auth';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';
import type {FluxAModelGroup} from '@/types/api';

type FluxAServiceTab = 'groups' | 'models';

function fluxaModelGroupsErrorMessage(error: unknown, t: ReturnType<typeof useI18n>['t']) {
  if (error instanceof Error && error.message.trim().toLowerCase() === 're-login required') {
    return t('fluxaModelGroups.reloginRequired');
  }
  return t('fluxaModelGroups.loadFailed');
}

function FluxAModelGroupSection({group}: {group: FluxAModelGroup}) {
  const {colors} = useAppTheme();
  return (
    <Pressable className="rounded-[18px] p-4" style={{backgroundColor: colors.surface}}>
      <View className="flex-row items-center justify-between">
        <View className="flex-1 flex-row items-center gap-2 pr-3">
          <Text style={{fontSize: fontSizes.md, fontWeight: '700', color: colors.textPrimary}}>{group.name}</Text>
          <View className="rounded-full bg-blue-500/15 px-2 py-1">
            <Text style={{fontSize: fontSizes.xs, color: '#2563EB'}}>{group.group || '用户分组'}</Text>
          </View>
        </View>
        <Ionicons name="chevron-forward" size={20} color={colors.textSecondary} />
      </View>
      <View className="mt-2 flex-row items-center gap-2">
        <Text style={{fontSize: fontSizes.sm, color: colors.textSecondary}}>{group.desc || ' '}</Text>
        <View className="rounded-full px-2 py-1" style={{backgroundColor: colors.brand + '22'}}>
          <Text style={{fontSize: fontSizes.xs, color: colors.brand}}>{group.ratio}x</Text>
        </View>
      </View>
    </Pressable>
  );
}

function ServiceTab({active, icon, label, onPress}: {active: boolean; icon: 'layers-outline' | 'hardware-chip-outline'; label: string; onPress: () => void}) {
  const {colors} = useAppTheme();
  return (
    <Pressable className="flex-1 flex-row items-center justify-center gap-2 rounded-[18px] py-3" onPress={onPress} style={{backgroundColor: active ? colors.surface : 'transparent'}}>
      <Ionicons name={icon} size={22} color={active ? colors.textPrimary : colors.textSecondary} />
      <Text style={{fontSize: fontSizes.md, fontWeight: active ? '600' : '500', color: active ? colors.textPrimary : colors.textSecondary}}>{label}</Text>
    </Pressable>
  );
}

export default function FluxAModelGroupsScreen() {
  const router = useRouter();
  const accessToken = useAuthStore((state) => state.accessToken);
  const user = useAuthStore((state) => state.user);
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const [activeTab, setActiveTab] = useState<FluxAServiceTab>('groups');
  const modelGroupsQuery = useQuery({
    queryKey: ['fluxa-model-groups', user?.id, user?.fluxaSite],
    queryFn: () => api.listFluxAModelGroups(accessToken!),
    enabled: Boolean(accessToken && user?.id && user?.fluxaSite),
  });
  const groups = modelGroupsQuery.data ?? [];
  const models = groups.flatMap((group) => group.models.map((model) => ({id: `${group.id}:${model.id}`, name: model.name, group: group.group})));

  return (
    <SafeAreaView className="flex-1" style={{backgroundColor: colors.background}} edges={['top', 'bottom', 'left', 'right']}>
      <View className="flex-row items-center justify-between px-3 pt-3">
        <Pressable className="h-11 w-11 items-center justify-center rounded-full" onPress={() => router.back()}>
          <Ionicons name="chevron-back" size={22} color={colors.textPrimary} />
        </Pressable>
        <Text style={{fontSize: fontSizes.lg, fontWeight: '500', color: colors.textPrimary}}>{t('fluxaModelGroups.title')}</Text>
        <View className="w-11" />
      </View>

      <ScrollView className="flex-1" contentContainerStyle={{padding: 20, paddingBottom: 28}} showsVerticalScrollIndicator={false}>
        <View className="mb-7 flex-row items-center gap-4">
          <View className="h-16 w-16 items-center justify-center rounded-[22px]" style={{backgroundColor: colors.surface}}>
            <FluxAIcon color={colors.brand} />
          </View>
          <View className="flex-1">
            <View className="flex-row items-center gap-2">
              <Text style={{fontSize: fontSizes.xl, fontWeight: '700', color: colors.textPrimary}}>FluxA</Text>
              {user?.fluxaGroup ? <View className="rounded-full bg-blue-500/15 px-2 py-1"><Text style={{fontSize: fontSizes.xs, color: '#2563EB'}}>{user.fluxaGroup}</Text></View> : null}
            </View>
            <Text className="mt-1" style={{fontSize: fontSizes.sm, color: colors.textSecondary}}>@{user?.username ?? ''}</Text>
          </View>
        </View>

        <View className="mb-6 flex-row rounded-[20px] p-1" style={{backgroundColor: colors.surfaceMuted}}>
          <ServiceTab active={activeTab === 'groups'} icon="layers-outline" label={t('fluxaModelGroups.groups')} onPress={() => setActiveTab('groups')} />
          <ServiceTab active={activeTab === 'models'} icon="hardware-chip-outline" label={t('fluxaModelGroups.models')} onPress={() => setActiveTab('models')} />
        </View>

        {activeTab === 'groups' ? (
          <View className="gap-3">
            <Text style={{fontSize: fontSizes.sm, lineHeight: 22, color: colors.textSecondary}}>{t('fluxaModelGroups.description')}</Text>
            {modelGroupsQuery.isLoading && !modelGroupsQuery.data ? (
              <View className="items-center gap-3 py-12"><ActivityIndicator size="large" color={colors.brand} /><Text style={{fontSize: fontSizes.sm, color: colors.textSecondary}}>{t('fluxaModelGroups.loading')}</Text></View>
            ) : modelGroupsQuery.error ? (
              <View className="items-center gap-4 py-12"><Text className="text-center" style={{fontSize: fontSizes.sm, lineHeight: 22, color: colors.textSecondary}}>{fluxaModelGroupsErrorMessage(modelGroupsQuery.error, t)}</Text><View className="min-w-[140px]"><PrimaryButton label={t('fluxaModelGroups.retry')} onPress={() => void modelGroupsQuery.refetch()} /></View></View>
            ) : groups.length === 0 ? (
              <Text style={{fontSize: fontSizes.sm, color: colors.textMuted}}>{t('fluxaModelGroups.empty')}</Text>
            ) : (
              groups.map((group) => <FluxAModelGroupSection key={group.id} group={group} />)
            )}
          </View>
        ) : (
          <View className="gap-3">
            <Text style={{fontSize: fontSizes.sm, lineHeight: 22, color: colors.textSecondary}}>{t('fluxaModelGroups.modelsDescription')}</Text>
            {models.length === 0 ? <Text style={{fontSize: fontSizes.sm, color: colors.textMuted}}>{t('fluxaModelGroups.modelsEmpty')}</Text> : models.map((model) => (
              <View key={model.id} className="flex-row items-center justify-between rounded-[18px] p-4" style={{backgroundColor: colors.surface}}>
                <Text className="flex-1 pr-3" style={{fontSize: fontSizes.md, fontWeight: '600', color: colors.textPrimary}}>{model.name}</Text>
                <View className="rounded-full bg-blue-500/15 px-2 py-1"><Text style={{fontSize: fontSizes.xs, color: '#2563EB'}}>{model.group || t('fluxaModelGroups.groups')}</Text></View>
              </View>
            ))}
          </View>
        )}

        <View className="mt-8 rounded-[18px] p-4" style={{backgroundColor: colors.surfaceMuted}}>
          <Text style={{fontSize: fontSizes.sm, lineHeight: 22, color: colors.textSecondary}}>{t('fluxaModelGroups.serviceDescription')}</Text>
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
