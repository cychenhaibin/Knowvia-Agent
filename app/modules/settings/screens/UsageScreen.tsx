import {Ionicons} from '@expo/vector-icons';
import {useRouter} from 'expo-router';
import {useMemo, useState} from 'react';
import {Pressable, ScrollView, Text, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {BottomSheet} from '@/components/BottomSheet';
import {useI18n} from '@/i18n/useI18n';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

type UsageSheetKey = 'points' | 'daily' | null;

type UsageHistoryGroup = {
  date: string;
  items: Array<{
    title: string;
    change: string;
  }>;
};

function UsageInfoSheet({
  visible,
  title,
  body,
  primaryLabel,
  secondaryLabel,
  onClose,
  onSecondaryPress,
}: {
  visible: boolean;
  title: string;
  body: string[];
  primaryLabel: string;
  secondaryLabel?: string;
  onClose: () => void;
  onSecondaryPress?: () => void;
}) {
  const {colors} = useAppTheme();

  return (
    <BottomSheet visible={visible} onClose={onClose}>
      <Text
        className="mb-8 text-center"
        style={{fontSize: 26, fontWeight: '700', color: colors.textPrimary}}>
        {title}
      </Text>
      <View className="gap-4">
        {body.map((line) => (
          <Text
            key={line}
            style={{
              fontSize: fontSizes.lg,
              lineHeight: 42,
              color: colors.textSecondary,
            }}>
            {line}
          </Text>
        ))}
      </View>
      <View className="mt-8 gap-4">
        <Pressable
          className="items-center rounded-[20px] py-5"
          style={{backgroundColor: colors.surfaceMuted}}
          onPress={onClose}>
          <Text
            style={{fontSize: 24, fontWeight: '600', color: colors.textPrimary}}>
            {primaryLabel}
          </Text>
        </Pressable>
        {secondaryLabel ? (
          <Pressable
            className="items-center rounded-[20px] py-5"
            style={{backgroundColor: colors.background}}
            onPress={onSecondaryPress}>
            <Text
              style={{fontSize: 24, fontWeight: '600', color: colors.textMuted}}>
              {secondaryLabel}
            </Text>
          </Pressable>
        ) : null}
      </View>
    </BottomSheet>
  );
}

export default function UsageScreen() {
  const router = useRouter();
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const [activeSheet, setActiveSheet] = useState<UsageSheetKey>(null);
  const usageHistory: UsageHistoryGroup[] = useMemo(
    () => [
      {
        date: '2026-04-19',
        items: [
          {title: '你好你是什么模型', change: '-4'},
          {title: '如何用用户名admin和密码admin123登录', change: '-58'},
        ],
      },
      {
        date: '2026-04-18',
        items: [
          {title: t('usage.historyDailyRefresh'), change: '+300'},
          {title: t('usage.historyFreeCredits'), change: '+2560'},
        ],
      },
    ],
    [t],
  );

  const pointsSheetBody = useMemo(
    () => [
      t('usage.note1'),
      t('usage.note2'),
      t('usage.note3'),
      t('usage.note4'),
      t('usage.note5'),
    ],
    [t],
  );

  const dailySheetBody = useMemo(() => [t('usage.dailyPlanLimit')], [t]);

  return (
    <>
      <SafeAreaView
        className="flex-1"
        style={{backgroundColor: colors.background}}
        edges={['top', 'bottom', 'left', 'right']}>
        <ScrollView
          contentContainerStyle={{paddingHorizontal: 24, paddingTop: 12, paddingBottom: 48}}
          showsVerticalScrollIndicator={false}>
          <View className="mb-10 flex-row items-center justify-between">
            <Pressable
              className="h-11 w-11 items-center justify-center rounded-full"
              onPress={() => router.back()}>
              <Ionicons name="chevron-back" size={28} color={colors.textPrimary} />
            </Pressable>
            <Text
              style={{fontSize: 30, fontWeight: '700', color: colors.textPrimary}}>
              {t('usage.title')}
            </Text>
            <View className="w-11" />
          </View>

          <View
            className="rounded-[22px] px-5 py-5"
            style={{
              backgroundColor: colors.surface,
              borderWidth: 1,
              borderColor: colors.border,
            }}>
            <View className="mb-5 flex-row items-center justify-between">
              <Text
                style={{
                  fontSize: 30,
                  fontWeight: '700',
                  color: colors.textPrimary,
                }}>
                {t('profile.free')}
              </Text>
              <Pressable
                className="rounded-[18px] px-8 py-4"
                style={{backgroundColor: colors.surfaceMuted}}
                onPress={() => router.push('/upgrade')}>
                <Text
                  style={{fontSize: 22, fontWeight: '600', color: colors.textPrimary}}>
                  {t('profile.upgrade')}
                </Text>
              </Pressable>
            </View>

            <View
              className="mb-6"
              style={{
                borderBottomWidth: 1,
                borderBottomColor: colors.divider,
                borderStyle: 'dashed',
              }}
            />

            <View className="gap-8">
              <View className="gap-4">
                <View className="flex-row items-center justify-between">
                  <View className="flex-row items-center gap-3">
                    <Ionicons name="sparkles-outline" size={28} color={colors.textPrimary} />
                    <Text
                      style={{fontSize: 22, fontWeight: '600', color: colors.textPrimary}}>
                      {t('profile.credits')}
                    </Text>
                    <Pressable
                      className="h-8 w-8 items-center justify-center rounded-full"
                      onPress={() => setActiveSheet('points')}>
                      <Ionicons
                        name="help-circle-outline"
                        size={24}
                        color={colors.iconSubtle}
                      />
                    </Pressable>
                  </View>
                  <Text
                    style={{fontSize: 24, fontWeight: '700', color: colors.textPrimary}}>
                    2560
                  </Text>
                </View>

                <View className="flex-row items-center justify-between pl-11">
                  <Text style={{fontSize: 18, color: colors.textMuted}}>
                    {t('usage.freeCredits')}
                  </Text>
                  <Text style={{fontSize: 18, color: colors.textMuted}}>2560</Text>
                </View>
              </View>

              <View className="gap-4">
                <View className="flex-row items-center justify-between">
                  <View className="flex-row items-center gap-3">
                    <Ionicons
                      name="calendar-clear-outline"
                      size={28}
                      color={colors.textPrimary}
                    />
                    <Text
                      style={{fontSize: 22, fontWeight: '600', color: colors.textPrimary}}>
                      {t('usage.dailyRefresh')}
                    </Text>
                    <Pressable
                      className="h-8 w-8 items-center justify-center rounded-full"
                      onPress={() => setActiveSheet('daily')}>
                      <Ionicons
                        name="help-circle-outline"
                        size={24}
                        color={colors.iconSubtle}
                      />
                    </Pressable>
                  </View>
                  <Text
                    style={{fontSize: 24, fontWeight: '700', color: colors.textPrimary}}>
                    300
                  </Text>
                </View>
              </View>
            </View>
          </View>

          <View className="mt-14 gap-6">
            <View className="flex-row items-center justify-between">
              <Text
                style={{fontSize: 30, fontWeight: '700', color: colors.textPrimary}}>
                {t('usage.history')}
              </Text>
              <View className="flex-row items-center gap-1">
                <Text style={{fontSize: 18, color: colors.textMuted}}>
                  {t('usage.timezone')}
                </Text>
                <Ionicons
                  name="help-circle-outline"
                  size={24}
                  color={colors.iconSubtle}
                />
              </View>
            </View>

            {usageHistory.map((group) => (
              <View key={group.date} className="gap-5">
                <Text style={{fontSize: 18, color: colors.textMuted}}>{group.date}</Text>
                {group.items.map((item, index) => (
                  <View key={`${group.date}-${item.title}`}>
                    <View className="flex-row items-center justify-between gap-4">
                      <Text
                        className="flex-1"
                        style={{
                          fontSize: 18,
                          lineHeight: 34,
                          color: colors.textPrimary,
                        }}>
                        {item.title}
                      </Text>
                      <Text
                        style={{fontSize: 18, fontWeight: '600', color: colors.textPrimary}}>
                        {item.change}
                      </Text>
                    </View>
                    {index === group.items.length - 1 ? (
                      <View
                        className="mt-6 h-px"
                        style={{backgroundColor: colors.divider}}
                      />
                    ) : null}
                  </View>
                ))}
              </View>
            ))}
          </View>
        </ScrollView>
      </SafeAreaView>

      <UsageInfoSheet
        visible={activeSheet === 'points'}
        title={t('usage.aboutCredits')}
        body={pointsSheetBody}
        primaryLabel={t('usage.knowIt')}
        secondaryLabel={t('usage.learnMore')}
        onClose={() => setActiveSheet(null)}
        onSecondaryPress={() => {
          setActiveSheet(null);
          router.push('/upgrade');
        }}
      />

      <UsageInfoSheet
        visible={activeSheet === 'daily'}
        title={t('usage.aboutDaily')}
        body={dailySheetBody}
        primaryLabel={t('usage.knowIt')}
        onClose={() => setActiveSheet(null)}
      />
    </>
  );
}
