import { AntDesign, MaterialCommunityIcons } from '@expo/vector-icons';
import { useRouter } from 'expo-router';
import { Linking, Pressable, Text, View } from 'react-native';
import { useState, type ReactNode } from 'react';
import Svg, { Path, Rect } from 'react-native-svg';

import { FluxAIcon } from '@/components/FluxAIcon';
import { Screen } from '@/components/Screen';
import { useTatos } from '@/components/Tatos';
import { useI18n } from '@/i18n/useI18n';
import { api } from '@/lib/api';
import { signInWithGoogle } from '@/lib/google-auth';
import { signInWithMicrosoft } from '@/lib/microsoft-auth';
import { useAuthStore } from '@/store/auth';
import type { AppColors } from '@/theme/colors';
import { useAppTheme } from '@/theme/useAppTheme';

const PROVIDER_LOGIN_URLS = {
  facebook: 'https://www.facebook.com/login/',
  microsoft: 'https://login.live.com/',
  apple: 'https://appleid.apple.com/sign-in',
} as const;

export default function LoginScreen() {
  const router = useRouter();
  const [googlePending, setGooglePending] = useState(false);
  const [microsoftPending, setMicrosoftPending] = useState(false);
  const { colors } = useAppTheme();
  const { t } = useI18n();
  const { showTatos, tatosNode } = useTatos();

  const openProviderLogin = async (
    provider: keyof typeof PROVIDER_LOGIN_URLS,
  ) => {
    try {
      const url = PROVIDER_LOGIN_URLS[provider];
      try {
        const WebBrowser = await import('expo-web-browser');
        const result = await WebBrowser.openBrowserAsync(url, {
          presentationStyle: WebBrowser.WebBrowserPresentationStyle.FORM_SHEET,
          showTitle: true,
          controlsColor: colors.brand,
          createTask: false,
        });
        if (result.type === 'cancel' || result.type === 'dismiss') {
          return;
        }
        return;
      } catch {
        const supported = await Linking.canOpenURL(url);
        if (!supported) {
          throw new Error(`unsupported provider url: ${provider}`);
        }
        await Linking.openURL(url);
      }
    } catch {
      showTatos({ body: t('login.providerOpenFailed') });
    }
  };

  const handleGoogleSignIn = async () => {
    try {
      setGooglePending(true);
      const result = await signInWithGoogle();
      const payload = await api.loginWithGoogle(result.idToken);
      await useAuthStore.getState().setSession(payload);
      router.replace('/(tabs)/runs');
    } catch (error) {
      const code =
        typeof error === 'object' && error !== null && 'code' in error
          ? String((error as { code?: unknown }).code)
          : '';
      if (code === 'GOOGLE_SIGN_IN_CANCELLED') {
        return;
      }

      const message =
        error instanceof Error && error.message
          ? error.message
          : t('login.googleFailure');
      showTatos({ body: message });
    } finally {
      setGooglePending(false);
    }
  };

  const handleMicrosoftSignIn = async () => {
    try {
      setMicrosoftPending(true);
      const result = await signInWithMicrosoft();
      const payload = await api.loginWithMicrosoft(result.idToken);
      await useAuthStore.getState().setSession(payload);
      router.replace('/(tabs)/runs');
    } catch (error) {
      const code =
        typeof error === 'object' && error !== null && 'code' in error
          ? String((error as { code?: unknown }).code)
          : '';
      if (code === 'MICROSOFT_SIGN_IN_CANCELLED') {
        return;
      }

      const message =
        error instanceof Error && error.message
          ? error.message
          : t('login.failure');
      showTatos({ body: message });
    } finally {
      setMicrosoftPending(false);
    }
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

          <View className="gap-20">

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
                label={`Facebook ${t('login.signIn')}`}
                icon={<FacebookLogo />}
                colors={colors}
                onPress={() => {
                  void openProviderLogin('facebook');
                }}
              />
            <LoginOption
              label={googlePending ? t('login.googleSigningIn') : `Google ${t('login.signIn')}`}
              icon={<GoogleLogo />}
              colors={colors}
              disabled={googlePending}
              onPress={() => {
                void handleGoogleSignIn();
              }}
            />
            <LoginOption
              label={microsoftPending ? t('login.signingIn') : `Microsoft ${t('login.signIn')}`}
              icon={<MicrosoftLogo />}
              colors={colors}
              disabled={microsoftPending}
              onPress={() => {
                void handleMicrosoftSignIn();
              }}
            />
            <LoginOption
              label={`Apple ${t('login.signIn')}`}
              icon={<AntDesign name="apple1" size={20} color={colors.textPrimary} />}
              colors={colors}
              onPress={() => {
                void openProviderLogin('apple');
              }}
            />

            <View className="mt-1 flex-row items-center gap-3">
              <View className="h-[1px] flex-1" style={{ backgroundColor: colors.divider }} />
              <Text className="text-lg" style={{ color: colors.textTertiary }}>
                {t('login.or')}
              </Text>
              <View className="h-[1px] flex-1" style={{ backgroundColor: colors.divider }} />
            </View>

            <LoginOption
              label={t('login.fluxaLogin')}
              icon={<FluxAIcon color={colors.textPrimary} />}
              colors={colors}
              onPress={() =>
                router.push({
                  pathname: '/(auth)/fluxa-login',
                  params: { openSitePicker: '1' },
                })
              }
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
  disabled,
  onPress,
}: {
  label: string;
  icon: ReactNode;
  colors: AppColors;
  disabled?: boolean;
  onPress?: () => void;
}) {
  return (
    <Pressable
      className="flex-row items-center rounded-[12px] border px-5 py-3"
      disabled={disabled}
      onPress={onPress}
      style={{
        backgroundColor: colors.surface,
        borderColor: colors.border,
        opacity: disabled ? 0.65 : 1,
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

function GoogleLogo() {
  return (
    <Svg width={18} height={18} viewBox="0 0 24 24" fill="none">
      <Path
        d="M23.766 12.277c0-.79-.064-1.58-.204-2.354H12v4.459h6.617a5.66 5.66 0 0 1-2.454 3.716v3.05h3.93c2.307-2.124 3.673-5.259 3.673-8.871Z"
        fill="#4285F4"
      />
      <Path
        d="M12 24c3.306 0 6.086-1.086 8.115-2.952l-3.93-3.05c-1.093.743-2.49 1.162-4.185 1.162-3.197 0-5.905-2.156-6.871-5.054H1.07v3.145A12.248 12.248 0 0 0 12 24Z"
        fill="#34A853"
      />
      <Path
        d="M5.129 14.106A7.347 7.347 0 0 1 4.746 12c0-.731.128-1.443.383-2.106V6.749H1.07A12.07 12.07 0 0 0 0 12c0 1.944.46 3.794 1.07 5.251l4.059-3.145Z"
        fill="#FBBC04"
      />
      <Path
        d="M12 4.839c1.782 0 3.35.612 4.603 1.817l3.433-3.432C18.083 1.4 15.304 0 12 0 7.31 0 3.273 2.69 1.07 6.749l4.059 3.145C6.095 6.995 8.803 4.839 12 4.839Z"
        fill="#EA4335"
      />
    </Svg>
  );
}

function FacebookLogo() {
  return (
    <Svg width={20} height={20} viewBox="0 0 24 24" fill="none">
      <Path
        d="M24 12.073C24 5.405 18.627 0 12 0S0 5.405 0 12.073c0 6.026 4.388 11.022 10.125 11.927v-8.437H7.078v-3.49h3.047V9.41c0-3.017 1.792-4.684 4.533-4.684 1.313 0 2.686.236 2.686.236v2.963H15.83c-1.49 0-1.955.931-1.955 1.886v2.262h3.328l-.532 3.49h-2.796V24C19.612 23.095 24 18.099 24 12.073Z"
        fill="#1877F2"
      />
      <Path
        d="M16.671 15.563l.532-3.49h-3.328V9.811c0-.955.465-1.886 1.955-1.886h1.514V4.962s-1.373-.236-2.686-.236c-2.741 0-4.533 1.667-4.533 4.684v2.663H7.078v3.49h3.047V24a12.17 12.17 0 0 0 3.75 0v-8.437h2.796Z"
        fill="#fff"
      />
    </Svg>
  );
}

function MicrosoftLogo() {
  return (
    <Svg width={20} height={20} viewBox="0 0 28 28" fill="none">
      <Rect x={2} y={2} width={10} height={10} fill="#F25022" />
      <Rect x={14} y={2} width={10} height={10} fill="#7FBA00" />
      <Rect x={2} y={14} width={10} height={10} fill="#00A4EF" />
      <Rect x={14} y={14} width={10} height={10} fill="#FFB900" />
    </Svg>
  );
}
