import { MaterialCommunityIcons } from '@expo/vector-icons';
import { Pressable, Text, View } from 'react-native';
import { type ReactNode } from 'react';

import { Screen } from '@/components/Screen';
import { useTatos } from '@/components/Tatos';
import { useI18n } from '@/i18n/useI18n';
import type { AppColors } from '@/theme/colors';
import { useAppTheme } from '@/theme/useAppTheme';

export default function LoginScreen() {
  const { colors } = useAppTheme();
  const { t } = useI18n();
  const { showTatos, tatosNode } = useTatos();

  const handleWeChatSignIn = () => {
    showTatos({
      title: '微信登录',
      body: '当前页面已切换为仅保留微信登录入口，微信鉴权尚未接入。',
    });
  };

  return (
    <>
      <Screen scroll>
        <View className="flex-1 justify-between pt-6">
          <View className="absolute inset-0 overflow-hidden">
            <DotCluster top={14} left={-10} color="rgba(120,120,120,0.12)" />
            <DotCluster top={62} right={34} color="rgba(70,110,255,0.25)" />
            <DotCluster top={255} left={90} color="rgba(255,120,120,0.18)" />
            <DotCluster bottom={170} right={80} color="rgba(255,170,70,0.18)" />
          </View>

          <View className="gap-80">
            <View className="items-center px-4 pt-12">
              <View className="items-center gap-8">
                <View className="h-24 w-24 items-center justify-center">
                  <MaterialCommunityIcons
                    name="gesture-tap-button"
                    size={84}
                    color={colors.textPrimary}
                  />
                </View>
                <Text
                  className="text-center text-[28px] font-semibold tracking-tight"
                  style={{ color: colors.textPrimary }}>
                  {t('login.welcome')}
                </Text>
              </View>
            </View>

            <View className="gap-4 px-4">
              <LoginOption
                label="微信登录"
                icon={<MaterialCommunityIcons name="wechat" size={22} color="#07C160" />}
                colors={colors}
                onPress={handleWeChatSignIn}
              />

              <Text
                className="px-3 pt-3 text-center text-sm"
                style={{ color: colors.textTertiary }}>
                {t('login.termsNotice')}
              </Text>
            </View>
          </View>
        </View>
      </Screen>

      {tatosNode}
    </>
  );
}

function LoginOption({
  label,
  icon,
  colors,
  onPress,
}: {
  label: string;
  icon: ReactNode;
  colors: AppColors;
  onPress?: () => void;
}) {
  return (
    <Pressable
      className="flex-row items-center rounded-[12px] border px-5 py-3"
      onPress={onPress}
      style={{
        backgroundColor: colors.surface,
        borderColor: colors.border,
      }}>
      <View className="w-12 items-start">{icon}</View>
      <View className="flex-1 items-center pr-6">
        <Text
          className="text-[16px] font-normal"
          style={{ color: colors.textPrimary }}>
          {label}
        </Text>
      </View>
    </Pressable>
  );
}

function DotCluster({
  color,
  top,
  right,
  bottom,
  left,
}: {
  color: string;
  top?: number;
  right?: number;
  bottom?: number;
  left?: number;
}) {
  return (
    <View
      pointerEvents="none"
      className="absolute flex-row flex-wrap"
      style={{
        top,
        right,
        bottom,
        left,
        width: 180,
        height: 180,
      }}>
      {Array.from({ length: 144 }).map((_, index) => (
        <View
          key={index}
          style={{
            width: 12,
            height: 12,
            alignItems: 'center',
            justifyContent: 'center',
          }}>
          <View
            style={{
              width: 2,
              height: 2,
              borderRadius: 999,
              backgroundColor: color,
            }}
          />
        </View>
      ))}
    </View>
  );
}
