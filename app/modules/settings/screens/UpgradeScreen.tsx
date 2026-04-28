import {Ionicons} from '@expo/vector-icons';
import {useRouter} from 'expo-router';
import type {ComponentProps} from 'react';
import {useMemo, useState} from 'react';
import {Pressable, Text, View} from 'react-native';

import {Screen} from '@/components/Screen';
import {useI18n} from '@/i18n/useI18n';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

type PlanKey = 'monthly' | 'yearly';

type PlanOption = {
  key: PlanKey;
  label: string;
  price: string;
  badge?: string;
};

type Benefit = {
  icon: ComponentProps<typeof Ionicons>['name'];
  text: string;
};

export default function UpgradeScreen() {
  const router = useRouter();
  const [selectedPlan, setSelectedPlan] = useState<PlanKey>('monthly');
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const plans: PlanOption[] = useMemo(
    () => [
      {
        key: 'monthly',
        label: t('upgrade.monthly'),
        price: '¥10',
        badge: t('upgrade.trialBadge'),
      },
      {
        key: 'yearly', 
        label: t('upgrade.yearly'), 
        price: '¥110',
        badge: t('upgrade.save20'),
      },
    ],
    [t],
  );
  const benefits: Benefit[] = useMemo(
    () => [
      // {icon: 'sparkles-outline', text: t('upgrade.benefit.points')},
      // {icon: 'search-outline', text: t('upgrade.benefit.research')},
      {icon: 'terminal-outline', text: t('upgrade.benefit.codex')},
      // {icon: 'browsers-outline', text: t('upgrade.benefit.deployment')},
      // {icon: 'albums-outline', text: t('upgrade.benefit.slides')},
      // {icon: 'speedometer-outline', text: t('upgrade.benefit.performance')},
      // {icon: 'list-outline', text: t('upgrade.benefit.parallel')},
    ],
    [t],
  );
  const selected = plans.find((plan) => plan.key === selectedPlan)!;

  return (
    <Screen scroll>
      <View className="gap-6">
        <View className="items-end">
          <Pressable
            className="h-11 w-11 items-center justify-center rounded-[12px]"
            style={{backgroundColor: colors.closeButtonBg}}
            onPress={() => router.back()}>
            <Ionicons name="close" size={24} color={colors.textMuted} />
          </Pressable>
        </View>

        <View className="px-2">
          <Text
            className="text-center font-semibold leading-[34px]"
            style={{fontSize: fontSizes.hero, color: colors.textPrimary}}>
            {t('upgrade.headline')}
          </Text>
        </View>

        <View
          className="gap-5 rounded-[12px] px-6 py-7"
          style={{backgroundColor: colors.surface}}>
          {benefits.map((benefit) => (
            <View key={benefit.text} className="flex-row items-start gap-4">
              <View className="h-8 w-8 items-center justify-center">
                <Ionicons name={benefit.icon} size={24} color={colors.textPrimary} />
              </View>
              <Text
                className="flex-1 leading-8"
                style={{fontSize: fontSizes.md, color: colors.textPrimary}}>
                {benefit.text}
              </Text>
            </View>
          ))}
        </View>

        <View className="gap-2">
          {plans.map((plan) => {
            const isSelected = selectedPlan === plan.key;

            return (
              <Pressable
                key={plan.key}
                className="flex-row items-center rounded-[12px] px-5 py-4"
                style={{
                  borderWidth: 1.5,
                  borderColor: isSelected ? colors.brand : colors.border,
                  backgroundColor: colors.surface,
                }}
                onPress={() => setSelectedPlan(plan.key)}>
                <View
                  className="mr-4 h-6 w-6 items-center justify-center rounded-full"
                  style={{
                    borderWidth: 1.5,
                    borderColor: isSelected ? colors.brand : colors.iconSubtle,
                  }}>
                  {isSelected ? (
                    <View
                      className="h-3.5 w-3.5 rounded-full"
                      style={{backgroundColor: colors.brand}}
                    />
                  ) : null}
                </View>
                <Text
                  className="flex-1 font-medium"
                  style={{fontSize: fontSizes.md, color: colors.textPrimary}}>
                  {plan.label}
                </Text>
                {plan.badge ? (
                  <View
                    className="mr-4 rounded-[16px] px-4 py-2"
                    style={{
                      backgroundColor: isSelected ? colors.brandSoft : 'transparent',
                      opacity: isSelected ? 1 : 0,
                    }}>
                    <Text
                      className="font-semibold"
                      style={{fontSize: fontSizes.xs, color: colors.brand}}>
                      {plan.badge}
                    </Text>
                  </View>
                ) : null}
                <Text
                  className="font-medium"
                  style={{fontSize: fontSizes.md, color: colors.textPrimary}}>
                  {plan.price}
                </Text>
              </Pressable>
            );
          })}
        </View>

        <Text
          className="text-center"
          style={{fontSize: fontSizes.xs, color: colors.textMuted}}>
          {selected.key === 'monthly'
            ? t('upgrade.monthlySummary')
            : t('upgrade.yearlySummary')}
        </Text>

        <Pressable
          className="items-center rounded-[12px] py-4"
          style={{backgroundColor: colors.brand}}>
          <Text
            className="font-semibold"
            style={{fontSize: fontSizes.md, color: colors.surface}}>
            {selected.key === 'monthly' ? t('upgrade.startTrial') : t('upgrade.upgradeYearly')}
          </Text>
        </Pressable>

        <View className="flex-row items-center justify-center gap-10">
          {[t('upgrade.terms'), t('upgrade.privacy'), t('upgrade.restore')].map((item) => (
            <Pressable key={item}>
              <Text style={{fontSize: fontSizes.xs, color: colors.textMuted}}>
                {item}
              </Text>
            </Pressable>
          ))}
        </View>
      </View>
    </Screen>
  );
}
