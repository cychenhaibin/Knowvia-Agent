import {useMutation} from '@tanstack/react-query';
import {useLocalSearchParams, useRouter} from 'expo-router';
import {useState} from 'react';
import {Pressable, Text, View} from 'react-native';

import {BottomSheet} from '@/components/BottomSheet';
import {PrimaryButton} from '@/components/PrimaryButton';
import {Screen} from '@/components/Screen';
import {TextField} from '@/components/TextField';
import {useI18n} from '@/i18n/useI18n';
import {api} from '@/lib/api';
import {useAuthStore} from '@/store/auth';
import {useAppTheme} from '@/theme/useAppTheme';
import type {FluxASite} from '@/types/api';

const fluxaSites: FluxASite[] = ['paid', 'free'];

class FluxA2FARequiredError extends Error {}

function isFluxASite(value: unknown): value is FluxASite {
  return value === 'paid' || value === 'free';
}

export function canSubmit(
  site: FluxASite | null,
  username: string,
  password: string,
  pending: boolean,
): boolean {
  return Boolean(site && username.trim() && password.trim() && !pending);
}

export function nextPasswordAfterSiteChange(
  previousSite: FluxASite | null,
  nextSite: FluxASite | null,
): string | null {
  return previousSite === nextSite ? null : '';
}

export default function FluxALoginScreen() {
  const router = useRouter();
  const {site: siteParam, openSitePicker} = useLocalSearchParams<{
    site?: string;
    openSitePicker?: string;
  }>();
  const initialSite = isFluxASite(siteParam) ? siteParam : null;
  const [site, setSite] = useState<FluxASite | null>(initialSite);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [showSitePicker, setShowSitePicker] = useState(
    initialSite === null || openSitePicker === '1',
  );
  const {colors} = useAppTheme();
  const {t} = useI18n();

  const mutation = useMutation({
    mutationFn: async () => {
      if (!site) {
        throw new Error('FluxA site is required');
      }

      const login = await api.loginWithFluxA(site, username.trim(), password);
      if (login.require2FA) {
        throw new FluxA2FARequiredError();
      }
      if (!login.accessToken) {
        throw new Error('FluxA verification failed');
      }

      return api.exchangeFluxASession(site, login.accessToken);
    },
    onSuccess: async (payload) => {
      await useAuthStore.getState().setSession(payload);
      router.replace('/(tabs)/runs');
    },
  });

  const selectSite = (nextSite: FluxASite) => {
    setPassword((currentPassword) =>
      nextPasswordAfterSiteChange(site, nextSite) ?? currentPassword,
    );
    setSite(nextSite);
    setShowSitePicker(false);
  };

  const errorMessage = mutation.error
    ? mutation.error instanceof FluxA2FARequiredError
      ? t('login.fluxa2FARequired')
      : t('login.fluxaVerificationFailure')
    : null;

  return (
    <Screen scroll>
      <View className="flex-1 gap-6 px-4 pt-10">
        <View className="gap-2">
          <Text
            className="text-[28px] font-semibold tracking-tight"
            style={{color: colors.textPrimary}}>
            {t('login.fluxaTitle')}
          </Text>
          <Text className="text-base" style={{color: colors.textSecondary}}>
            {t('login.fluxaChooseSiteHint')}
          </Text>
        </View>

        <View className="gap-4">
          <View className="gap-2">
            <Text className="text-md font-semibold" style={{color: colors.textPrimary}}>
              {t('login.fluxaSiteSelector')}
            </Text>
            <Pressable
              className="rounded-xl border px-4 py-3"
              disabled={mutation.isPending}
              onPress={() => setShowSitePicker(true)}
              style={{
                backgroundColor: colors.surface,
                borderColor: colors.border,
                opacity: mutation.isPending ? 0.75 : 1,
              }}>
              <Text style={{color: site ? colors.textPrimary : colors.textTertiary}}>
                {site ? t(`login.fluxaSite.${site}`) : t('login.fluxaChooseSiteHint')}
              </Text>
            </Pressable>
          </View>

          <TextField
            label={t('login.username')}
            value={username}
            onChangeText={setUsername}
            placeholder={t('login.username')}
            editable={site !== null}
          />
          <TextField
            label={t('login.password')}
            value={password}
            onChangeText={setPassword}
            placeholder={t('login.password')}
            secureTextEntry
            editable={site !== null}
          />

          <PrimaryButton
            label={mutation.isPending ? t('login.signingIn') : t('login.signIn')}
            disabled={!canSubmit(site, username, password, mutation.isPending)}
            onPress={() => mutation.mutate()}
          />

          {errorMessage ? (
            <Text className="text-center text-sm leading-6" style={{color: colors.textSecondary}}>
              {errorMessage}
            </Text>
          ) : null}
        </View>
      </View>

      <BottomSheet
        visible={showSitePicker}
        onClose={() => setShowSitePicker(false)}
        title={t('login.fluxaSiteSelector')}>
        <View className="gap-3">
          {fluxaSites.map((option) => (
            <Pressable
              key={option}
              className="rounded-[18px] p-4"
              disabled={mutation.isPending}
              onPress={() => selectSite(option)}
              style={{
                backgroundColor: option === site ? colors.surfaceMuted : colors.surface,
                borderColor: colors.border,
                borderWidth: 1,
                opacity: mutation.isPending ? 0.75 : 1,
              }}>
              <Text className="text-base font-semibold" style={{color: colors.textPrimary}}>
                {t(`login.fluxaSite.${option}`)}
              </Text>
            </Pressable>
          ))}
          {site === null ? (
            <Text className="text-sm" style={{color: colors.textTertiary}}>
              {t('login.fluxaSiteUnavailable')}
            </Text>
          ) : null}
        </View>
      </BottomSheet>
    </Screen>
  );
}
