import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {dirname} from 'node:path';
import Module from 'node:module';
import test from 'node:test';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import type {ComponentType, ReactNode} from 'react';
import typescript from 'typescript';

import type {signInWithGoogle} from '../lib/google-auth';

import {getDictionary} from '../i18n/messages';
import type {api as ApiClient} from '../lib/api';
import {formatFluxABalance} from '../lib/fluxaBalance';
import {appThemes} from '../theme/colors';
import type {FluxABalance, FluxASite, SessionPayload} from '../types/api';
import {
  FluxA2FARequiredError,
  loginThroughFluxA,
  nextPasswordAfterSiteChange,
  canSubmit,
  credentialsAreEditable,
} from '../modules/auth/fluxaFlow';

test('Google sign-in adapter needs no caller-provided OAuth client ID', () => {
  type GoogleSignInParameters = Parameters<typeof signInWithGoogle>;
  type RequiresNoArguments = GoogleSignInParameters extends [] ? true : false;
  const requiresNoArguments: RequiresNoArguments = true;
  assert.equal(requiresNoArguments, true);
});

test('Chinese FluxA labels identify the service as the transit station', () => {
  const messages = getDictionary('zh-Hans');
  assert.equal(messages['login.fluxaLogin'], 'FluxA中转站登录');
  assert.equal(messages['login.fluxaTitle'], 'FluxA中转站登录');
});

test('FluxA login uses full-width fields without a credential card and names the free site public', () => {
  const messages = getDictionary('zh-Hans');
  const screen = readFileSync('modules/auth/screens/FluxALoginScreen.tsx', 'utf8');

  assert.equal(messages['login.fluxaSite.free'], 'FluxA 公益站');
  assert.match(screen, /className="w-full gap-4"/);
  assert.doesNotMatch(screen, /className="gap-4 rounded-\[18px\] p-3"/);
});

test('credentials stay gated until a FluxA site is selected', () => {
  assert.equal(credentialsAreEditable(null), false);
  assert.equal(credentialsAreEditable('paid'), true);
  assert.equal(credentialsAreEditable('free'), true);
  assert.equal(canSubmit(null, 'user', 'secret', false), false);
  assert.equal(canSubmit('paid', 'user', 'secret', false), true);
  assert.equal(canSubmit('paid', 'user', 'secret', true), false);
});

test('switching FluxA sites clears the password while keeping same-site edits', () => {
  assert.equal(nextPasswordAfterSiteChange('paid', 'free'), '');
  assert.equal(nextPasswordAfterSiteChange('free', 'paid'), '');
  assert.equal(nextPasswordAfterSiteChange('paid', 'paid'), null);
});

test('single backend FluxA login trims only the username and preserves password bytes', async () => {
  const calls: string[] = [];
  const saved: unknown[] = [];
  const result = await loginThroughFluxA(
    {site: 'free', username: ' user ', password: ' secret\t '},
    {
      loginWithFluxA: async (
        site: FluxASite,
        username: string,
        password: string,
      ): Promise<SessionPayload> => {
        calls.push(`login:${site}:${username}:${password}`);
        return {
          accessToken: 'knowvia-access',
          refreshToken: 'knowvia-refresh',
          expiresIn: 3600,
          user: {id: '1', username: 'fluxa-free-42', displayName: 'FluxA User', fluxaSite: 'free'},
        };
      },
      persistSession: async (session: SessionPayload) => {
        saved.push(session);
      },
    },
  );

  assert.deepEqual(calls, ['login:free:user: secret\t ']);
  assert.equal(JSON.stringify(calls).includes('origin'), false);
  assert.equal(JSON.stringify(calls).includes('accessToken'), false);
  assert.equal(JSON.stringify(calls).includes('exchange'), false);
  assert.equal(result.user.fluxaSite, 'free');
  assert.deepEqual(saved, [result]);
});

test('FluxA session persists its site but ordinary sessions do not', () => {
  const fluxaSession: SessionPayload = {
    accessToken: 'knowvia-access',
    refreshToken: 'knowvia-refresh',
    expiresIn: 3600,
    user: {id: '1', username: 'fluxa-paid-42', displayName: 'FluxA User', fluxaSite: 'paid'},
  };
  const passwordSession: SessionPayload = {
    accessToken: 'knowvia-access',
    refreshToken: 'knowvia-refresh',
    expiresIn: 3600,
    user: {id: '2', username: 'password-user', displayName: 'Password User'},
  };

  assert.equal(fluxaSession.user.fluxaSite, 'paid');
  assert.equal(passwordSession.user.fluxaSite, undefined);
});

test('FluxA login renders the local RetroArch SVG', () => {
  const login = readFileSync('modules/auth/screens/LoginScreen-en.tsx', 'utf8');
  assert.match(login, /<FluxAIcon/);
  assert.doesNotMatch(login, /cdn\.simpleicons\.org/);
});

test('FluxA icon accepts optional dimensions with a 20 pixel default', () => {
  const icon = readFileSync('components/FluxAIcon.tsx', 'utf8');

  assert.match(icon, /width\?: number/);
  assert.match(icon, /height\?: number/);
  assert.match(icon, /width = 20/);
  assert.match(icon, /height = 20/);
});

test('model group settings are gated by fluxaSite', () => {
  const profile = readFileSync('modules/profile/screens/ProfileScreen.tsx', 'utf8');
  assert.match(profile, /user\?\.fluxaSite/);
  assert.match(profile, /router\.push\('\/fluxa-model-groups'\)/);
});

test('FluxA model group cache is scoped to the signed-in account and site', () => {
  const screen = readFileSync('modules/settings/screens/FluxAModelGroupsScreen.tsx', 'utf8');
  const auth = readFileSync('store/auth.ts', 'utf8');

  assert.match(screen, /queryKey:\s*\['fluxa-model-groups',\s*user\?\.id,\s*user\?\.fluxaSite\]/);
  assert.doesNotMatch(screen, /queryKey:\s*\['fluxa-model-groups'\]/);
  assert.match(auth, /queryClient\.removeQueries\(\{queryKey:\s*\['fluxa-model-groups'\]\}\)/);
});

test('FluxA model selectors reuse a stable empty array when no models are configured', () => {
  const preferences = readFileSync('store/preferences.ts', 'utf8');
  const runs = readFileSync('modules/chat/screens/RunsScreen.tsx', 'utf8');
  const modelGroups = readFileSync('modules/settings/screens/FluxAModelGroupsScreen.tsx', 'utf8');

  assert.match(preferences, /export const EMPTY_ENABLED_FLUXA_MODELS: EnabledFluxAModel\[\] = \[\];/);
  assert.match(runs, /import \{EMPTY_ENABLED_FLUXA_MODELS, usePreferencesStore\} from '@\/store\/preferences';/);
  assert.match(runs, /enabledFluxAModelsByUser\[user\?\.id \?\? ''\] \?\? EMPTY_ENABLED_FLUXA_MODELS/);
  assert.match(modelGroups, /import \{EMPTY_ENABLED_FLUXA_MODELS, usePreferencesStore\} from '@\/store\/preferences';/);
  assert.match(modelGroups, /enabledFluxAModelsByUser\[user\?\.id \?\? ''\] \?\? EMPTY_ENABLED_FLUXA_MODELS/);
});

test('profile balance query is scoped to the active FluxA account and site', () => {
  const screen = readFileSync('modules/profile/screens/ProfileScreen.tsx', 'utf8');

  assert.match(screen, /queryKey:\s*\['fluxa-balance',\s*user\?\.id,\s*user\?\.fluxaSite\]/);
  assert.match(screen, /enabled:\s*Boolean\(accessToken && user\?\.id && user\?\.fluxaSite\)/);
  assert.match(screen, /api\.getFluxABalance\(accessToken!\)/);
  assert.match(screen, /formatFluxABalance\(balanceQuery\.data, locale\)/);
});

test('FluxA balance group configuration maps supported plans and omits unsupported groups', () => {
  const apiTypes = readFileSync('types/api.ts', 'utf8');
  const {resolveFluxAPlan} = loadProfileScreenModuleForBalanceStateTest({
    getFluxABalance: async () => {
      throw new Error('not used by this mapping test');
    },
  });

  assert.match(apiTypes, /export interface FluxABalance \{[\s\S]*?group:\s*string;/);
  assert.ok(resolveFluxAPlan);
  assert.deepEqual(resolveFluxAPlan('default'), {titleKey: 'profile.free'});
  assert.deepEqual(resolveFluxAPlan('vip'), {
    titleKey: 'profile.subscription',
    badgeLabel: 'vip',
  });
  assert.deepEqual(resolveFluxAPlan('svip'), {
    titleKey: 'profile.subscription',
    badgeLabel: 'svip',
  });
  assert.deepEqual(resolveFluxAPlan('ssvip'), {
    titleKey: 'profile.subscription',
    badgeLabel: 'ssvip',
  });
  assert.equal(resolveFluxAPlan(''), undefined);
  assert.equal(resolveFluxAPlan('unknown'), undefined);

  const messages: Record<string, string> = getDictionary('zh-Hans');
  assert.equal(messages['profile.free'], '免费版');
  assert.equal(messages['profile.subscription'], '订阅版');
});

test('profile trims balance groups before rendering the configured plan', () => {
  const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}});
  const {ProfileScreen} = loadProfileScreenModuleForBalanceStateTest({
    getFluxABalance: async () => {
      throw new Error('query cache should satisfy this render');
    },
  });

  try {
    queryClient.setQueryData(['fluxa-balance', 'user-1', 'paid'], {
      group: ' svip ',
      quota: 7_400_000,
      quotaPerUnit: 500_000,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7.2,
      customCurrencySymbol: '',
      customCurrencyExchangeRate: 1,
    } as unknown as FluxABalance);

    const profile = renderProfileScreen(ProfileScreen, queryClient);
    assert.match(profile, /订阅版/);
    assert.match(profile, /svip/);
  } finally {
    queryClient.clear();
  }
});

test('profile plan header keeps Japanese subscription controls within a shrinkable, wrapping title area', () => {
  const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}});
  const {ProfileScreen} = loadProfileScreenModuleForBalanceStateTest(
    {
      getFluxABalance: async () => {
        throw new Error('query cache should satisfy this render');
      },
    },
    {
      language: 'ja',
      translations: {
        'profile.subscription': 'サブスクリプション',
        'profile.upgrade': 'アップグレード',
      },
    },
  );

  try {
    queryClient.setQueryData(['fluxa-balance', 'user-1', 'paid'], {
      group: 'ssvip',
      quota: 7_400_000,
      quotaPerUnit: 500_000,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7.2,
      customCurrencySymbol: '',
      customCurrencyExchangeRate: 1,
    } as unknown as FluxABalance);

    const profile = renderProfileScreen(ProfileScreen, queryClient);
    assert.match(profile, /サブスクリプション/);
    assert.match(profile, /ssvip/);
    assert.match(profile, /アップグレード/);
    assert.match(profile, /data-layout="min-w-0 flex-1 flex-row flex-wrap items-center gap-2 pr-2"/);
    assert.match(profile, /data-layout="shrink-0 rounded-xl px-4 py-2"/);
    assert.match(profile, /data-layout="shrink"/);
  } finally {
    queryClient.clear();
  }
});

test('profile plan badge uses AA-compliant theme colors in light and dark themes', () => {
  for (const colors of Object.values(appThemes)) {
    assert.ok(
      contrastRatio(colors.textPrimary, colors.surfaceMuted) >= 4.5,
      `${colors.textPrimary} on ${colors.surfaceMuted} must meet 4.5:1 contrast`,
    );

    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const {ProfileScreen} = loadProfileScreenModuleForBalanceStateTest(
      {
        getFluxABalance: async () => {
          throw new Error('query cache should satisfy this render');
        },
      },
      {colors},
    );

    try {
      queryClient.setQueryData(['fluxa-balance', 'user-1', 'paid'], {
        group: 'vip',
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7.2,
        customCurrencySymbol: '',
        customCurrencyExchangeRate: 1,
      } as unknown as FluxABalance);

      const profile = renderProfileScreen(ProfileScreen, queryClient);
      assert.match(profile, new RegExp(`data-background-color="${colors.surfaceMuted}"`));
      assert.match(profile, new RegExp(`<span data-color="${colors.textPrimary}">vip</span>`));
    } finally {
      queryClient.clear();
    }
  }
});

test('profile displays the formatted balance after the FluxA group and uses it for credits', () => {
  const screen = readFileSync('modules/profile/screens/ProfileScreen.tsx', 'utf8');
  const groupPillIndex = screen.indexOf('user?.fluxaGroup');
  const balancePillIndex = screen.indexOf('formattedBalance ?');

  assert.ok(groupPillIndex >= 0);
  assert.ok(balancePillIndex > groupPillIndex);
  assert.match(screen, /\{formattedBalance \? \([\s\S]*?\{formattedBalance\}[\s\S]*?\) : null\}/);
  assert.match(screen, /creditsValue=\{formattedBalance\}/);
  assert.match(screen, /creditsValue\?: string \| null/);
  assert.doesNotMatch(screen, />2860</);
});

test('profile hides cached FluxA balance when its refresh fails', async () => {
  const balance: FluxABalance = {
    group: 'default',
    quota: 7_400_000,
    quotaPerUnit: 500_000,
    quotaDisplayType: 'CNY',
    usdExchangeRate: 7.2,
    customCurrencySymbol: '',
    customCurrencyExchangeRate: 1,
  };
  const queryKey = ['fluxa-balance', 'user-1', 'paid'];
  const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}});
  let nextResponse: FluxABalance | Error = balance;
  const apiClient = {
    getFluxABalance: async () => {
      if (nextResponse instanceof Error) throw nextResponse;
      return nextResponse;
    },
  };
  const {ProfileScreen} = loadProfileScreenModuleForBalanceStateTest(apiClient);

  try {
    await queryClient.fetchQuery({
      queryKey,
      queryFn: () => apiClient.getFluxABalance(),
    });
    nextResponse = new Error('FluxA balance refresh failed');
    await assert.rejects(
      queryClient.fetchQuery({
        queryKey,
        queryFn: () => apiClient.getFluxABalance(),
      }),
    );

    const cachedErrorState = queryClient.getQueryState<FluxABalance>(queryKey);
    assert.deepEqual(cachedErrorState?.data, balance);
    assert.equal(cachedErrorState?.status, 'error');

    const refreshFailure = renderProfileScreen(ProfileScreen, queryClient);
    assert.doesNotMatch(refreshFailure, /¥106\.56/);
    assert.doesNotMatch(refreshFailure, /免费版|订阅版|vip|svip|ssvip/);
  } finally {
    queryClient.clear();
  }
});

test('logout removes all cached FluxA balances', () => {
  const auth = readFileSync('store/auth.ts', 'utf8');

  assert.match(auth, /queryClient\.removeQueries\(\{queryKey:\s*\['fluxa-balance'\]\}\)/);
});

test('FluxA model group cards use the unique token identifier as their React key', () => {
  const screen = readFileSync('modules/settings/screens/FluxAModelGroupsScreen.tsx', 'utf8');

  assert.match(screen, /FluxAModelGroupSection key=\{group\.id\}/);
  assert.doesNotMatch(screen, /FluxAModelGroupSection key=\{group\.name\}/);
});

test('formats FluxA CNY quota using the configured USD exchange rate', () => {
  assert.equal(
    formatFluxABalance(
      {
        group: 'default',
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7.2,
        customCurrencySymbol: '',
        customCurrencyExchangeRate: 1,
      },
      'zh-CN',
    ),
    '¥106.56',
  );
});

test('formats FluxA USD quota with a currency symbol', () => {
  assert.equal(
    formatFluxABalance(
      {
        group: 'default',
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'USD',
        usdExchangeRate: 7.2,
        customCurrencySymbol: '',
        customCurrencyExchangeRate: 1,
      },
      'en-US',
    ),
    '$14.80',
  );
});

test('formats FluxA custom quota with its configured symbol and exchange rate', () => {
  assert.equal(
    formatFluxABalance(
      {
        group: 'default',
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'CUSTOM',
        usdExchangeRate: 1,
        customCurrencySymbol: 'K',
        customCurrencyExchangeRate: 2,
      },
      'en-US',
    ),
    'K29.6',
  );
});

test('formats FluxA token quota without currency conversion', () => {
  assert.equal(
    formatFluxABalance(
      {
        group: 'default',
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'TOKENS',
        usdExchangeRate: 1,
        customCurrencySymbol: '',
        customCurrencyExchangeRate: 1,
      },
      'en-US',
    ),
    '7,400,000',
  );
});

test('returns null for invalid FluxA balance values', () => {
  const invalidBalance: FluxABalance = {
    group: 'default',
    quota: 7_400_000,
    quotaPerUnit: 0,
    quotaDisplayType: 'USD',
    usdExchangeRate: 7.2,
    customCurrencySymbol: '',
    customCurrencyExchangeRate: 1,
  };

  assert.equal(formatFluxABalance(invalidBalance), null);
});

test('formats FluxA balances when unrelated conversion settings are invalid', () => {
  assert.equal(
    formatFluxABalance(
      {
        group: 'default',
        quota: 7_400_000,
        quotaPerUnit: 0,
        quotaDisplayType: 'TOKENS',
        usdExchangeRate: Number.NaN,
        customCurrencySymbol: '',
        customCurrencyExchangeRate: Number.POSITIVE_INFINITY,
      },
      'en-US',
    ),
    '7,400,000',
  );
  assert.equal(
    formatFluxABalance(
      {
        group: 'default',
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'USD',
        usdExchangeRate: Number.NaN,
        customCurrencySymbol: '',
        customCurrencyExchangeRate: Number.POSITIVE_INFINITY,
      },
      'en-US',
    ),
    '$14.80',
  );
  assert.equal(
    formatFluxABalance(
      {
        group: 'default',
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7.2,
        customCurrencySymbol: '',
        customCurrencyExchangeRate: Number.POSITIVE_INFINITY,
      },
      'zh-CN',
    ),
    '¥106.56',
  );
  assert.equal(
    formatFluxABalance(
      {
        group: 'default',
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'CUSTOM',
        usdExchangeRate: Number.POSITIVE_INFINITY,
        customCurrencySymbol: 'K',
        customCurrencyExchangeRate: 2,
      },
      'en-US',
    ),
    'K29.6',
  );
});

test('formats FluxA balances when unrelated conversion settings are absent', () => {
  assert.equal(
    formatFluxABalance(
      {quota: 7_400_000, quotaDisplayType: 'TOKENS'} as unknown as FluxABalance,
      'en-US',
    ),
    '7,400,000',
  );
  assert.equal(
    formatFluxABalance(
      {
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'USD',
      } as unknown as FluxABalance,
      'en-US',
    ),
    '$14.80',
  );
  assert.equal(
    formatFluxABalance(
      {
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7.2,
      } as unknown as FluxABalance,
      'zh-CN',
    ),
    '¥106.56',
  );
  assert.equal(
    formatFluxABalance(
      {
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'CUSTOM',
        customCurrencySymbol: 'K',
        customCurrencyExchangeRate: 2,
      } as unknown as FluxABalance,
      'en-US',
    ),
    'K29.6',
  );
});

test('returns null when a FluxA balance lacks a display type required input', () => {
  assert.equal(
    formatFluxABalance({
      group: 'default',
      quota: Number.NaN,
      quotaPerUnit: 0,
      quotaDisplayType: 'TOKENS',
      usdExchangeRate: 1,
      customCurrencySymbol: '',
      customCurrencyExchangeRate: 1,
    }),
    null,
  );
  assert.equal(
    formatFluxABalance({
      group: 'default',
      quota: 7_400_000,
      quotaPerUnit: 0,
      quotaDisplayType: 'USD',
      usdExchangeRate: 1,
      customCurrencySymbol: '',
      customCurrencyExchangeRate: 1,
    }),
    null,
  );
  assert.equal(
    formatFluxABalance({
      group: 'default',
      quota: 7_400_000,
      quotaPerUnit: 500_000,
      quotaDisplayType: 'CNY',
      usdExchangeRate: Number.POSITIVE_INFINITY,
      customCurrencySymbol: '',
      customCurrencyExchangeRate: 1,
    }),
    null,
  );
  assert.equal(
    formatFluxABalance({
      group: 'default',
      quota: 7_400_000,
      quotaPerUnit: 500_000,
      quotaDisplayType: 'CUSTOM',
      usdExchangeRate: 1,
      customCurrencySymbol: 'K',
      customCurrencyExchangeRate: Number.NaN,
    }),
    null,
  );
});

test('FluxA service details switch between configuration and account groups without credential fields', () => {
  const screen = readFileSync('modules/settings/screens/FluxAModelGroupsScreen.tsx', 'utf8');

  assert.match(screen, /useState<FluxAServiceTab>\('groups'\)/);
  assert.match(screen, /fluxaModelGroups\.groups/);
  assert.match(screen, /fluxaModelGroups\.models/);
  assert.match(screen, /fluxaModelGroups\.serviceDescription/);
  assert.doesNotMatch(screen, /API Key|API代理地址|连通性检查/);
});

test('FluxA model tab loads models through the group endpoint', () => {
  const screen = readFileSync('modules/settings/screens/FluxAModelGroupsScreen.tsx', 'utf8');

  assert.match(screen, /api\.listFluxAModels\(accessToken!, group\)/);
  assert.match(screen, /activeTab === 'models'/);
});

test('FluxA model rows use the group and model identifier as a unique key', () => {
  const screen = readFileSync('modules/settings/screens/FluxAModelGroupsScreen.tsx', 'utf8');

  assert.match(screen, /key=\{`\$\{model\.group\}:\$\{model\.id\}`\}/);
  assert.doesNotMatch(screen, /<View key=\{model\.id\}/);
});

test('FluxA model switches persist enabled display preferences and feed the home picker', () => {
  const settings = readFileSync('modules/settings/screens/FluxAModelGroupsScreen.tsx', 'utf8');
  const preferences = readFileSync('store/preferences.ts', 'utf8');
  const menu = readFileSync('modules/chat/components/ModelMenu.tsx', 'utf8');
  const runs = readFileSync('modules/chat/screens/RunsScreen.tsx', 'utf8');

  assert.match(settings, /<Switch/);
  assert.match(settings, /setEnabledFluxAModels/);
  assert.match(preferences, /FLUXA_MODELS_KEY/);
  assert.match(menu, /fluxaModels/);
  assert.match(runs, /fluxaModels=\{enabledFluxAModels\}/);
});

test('model group API sends only the Knowvia bearer token', async () => {
  const originalFetch = globalThis.fetch;
  const originalBaseUrl = process.env.EXPO_PUBLIC_API_BASE_URL;
  const originalDev = (globalThis as {__DEV__?: boolean}).__DEV__;
  let fetchCall: {url: string; headers: Headers} | undefined;
  process.env.EXPO_PUBLIC_API_BASE_URL = 'https://knowvia.example/v1';
  (globalThis as {__DEV__?: boolean}).__DEV__ = false;
  globalThis.fetch = async (input, init) => {
    fetchCall = {url: String(input), headers: new Headers(init?.headers)};
    return new Response('[]', {
      status: 200,
      headers: {'Content-Type': 'application/json'},
    });
  };

  try {
    await loadApiForTest().listFluxAModelGroups('knowvia-token');
  } finally {
    globalThis.fetch = originalFetch;
    if (originalBaseUrl === undefined) {
      delete process.env.EXPO_PUBLIC_API_BASE_URL;
    } else {
      process.env.EXPO_PUBLIC_API_BASE_URL = originalBaseUrl;
    }
    (globalThis as {__DEV__?: boolean}).__DEV__ = originalDev;
  }

  assert.ok(fetchCall);
  assert.equal(fetchCall.url, 'https://knowvia.example/v1/fluxa/model-groups');
  assert.equal(fetchCall.headers.get('Authorization'), 'Bearer knowvia-token');
});

test('FluxA balance API sends only the Knowvia bearer token', async () => {
  const originalFetch = globalThis.fetch;
  const originalBaseUrl = process.env.EXPO_PUBLIC_API_BASE_URL;
  const originalDev = (globalThis as {__DEV__?: boolean}).__DEV__;
  let fetchCall: {url: string; headers: Headers} | undefined;
  process.env.EXPO_PUBLIC_API_BASE_URL = 'https://knowvia.example/v1';
  (globalThis as {__DEV__?: boolean}).__DEV__ = false;
  globalThis.fetch = async (input, init) => {
    fetchCall = {url: String(input), headers: new Headers(init?.headers)};
    return new Response(
      JSON.stringify({
        quota: 7_400_000,
        quotaPerUnit: 500_000,
        quotaDisplayType: 'USD',
        usdExchangeRate: 7.2,
        customCurrencySymbol: '',
        customCurrencyExchangeRate: 1,
      }),
      {status: 200, headers: {'Content-Type': 'application/json'}},
    );
  };

  try {
    await loadApiForTest().getFluxABalance('knowvia-token');
  } finally {
    globalThis.fetch = originalFetch;
    if (originalBaseUrl === undefined) {
      delete process.env.EXPO_PUBLIC_API_BASE_URL;
    } else {
      process.env.EXPO_PUBLIC_API_BASE_URL = originalBaseUrl;
    }
    (globalThis as {__DEV__?: boolean}).__DEV__ = originalDev;
  }

  assert.ok(fetchCall);
  assert.equal(fetchCall.url, 'https://knowvia.example/v1/fluxa/balance');
  assert.equal(fetchCall.headers.get('Authorization'), 'Bearer knowvia-token');
});

test('model API encodes the selected FluxA group', async () => {
  const originalFetch = globalThis.fetch;
  const originalBaseUrl = process.env.EXPO_PUBLIC_API_BASE_URL;
  let requestedUrl = '';
  process.env.EXPO_PUBLIC_API_BASE_URL = 'https://knowvia.example/v1';
  globalThis.fetch = async (input) => {
    requestedUrl = String(input);
    return new Response('[]', {status: 200, headers: {'Content-Type': 'application/json'}});
  };
  try {
    await loadApiForTest().listFluxAModels('knowvia-token', 'gpt 专用');
  } finally {
    globalThis.fetch = originalFetch;
    if (originalBaseUrl === undefined) delete process.env.EXPO_PUBLIC_API_BASE_URL;
    else process.env.EXPO_PUBLIC_API_BASE_URL = originalBaseUrl;
  }
  assert.equal(requestedUrl, 'https://knowvia.example/v1/fluxa/models?group=gpt%20%E4%B8%93%E7%94%A8');
});

function renderProfileScreen(ProfileScreen: ComponentType, queryClient?: QueryClient) {
  const React = require('react') as typeof import('react');
  const {renderToStaticMarkup} = require('react-dom/server') as {
    renderToStaticMarkup: (element: ReturnType<typeof React.createElement>) => string;
  };

  const profile = React.createElement(ProfileScreen);
  const screen = queryClient
    ? React.createElement(QueryClientProvider, {client: queryClient}, profile)
    : profile;

  return renderToStaticMarkup(screen);
}

function loadProfileScreenModuleForBalanceStateTest(
  apiClient: {getFluxABalance: () => Promise<FluxABalance>},
  options: {
    language?: string;
    translations?: Record<string, string>;
    colors?: Record<string, string>;
  } = {},
) {
  const React = require('react') as typeof import('react');
  const language = options.language ?? 'zh-Hans';
  const translations = options.translations ?? {};
  const colors = options.colors ?? {
    surface: '#fff',
    surfaceMuted: '#eee',
    textPrimary: '#111',
    brand: '#f00',
    divider: '#ddd',
    icon: '#111',
    iconSubtle: '#999',
    textMuted: '#777',
    textSecondary: '#666',
    textTertiary: '#555',
    overlaySoft: '#000',
    shadow: '#000',
  };
  const profilePath = 'modules/profile/screens/ProfileScreen.tsx';
  const source = readFileSync(profilePath, 'utf8');
  const compiled = typescript.transpileModule(source, {
    compilerOptions: {
      jsx: typescript.JsxEmit.ReactJSX,
      module: typescript.ModuleKind.CommonJS,
      target: typescript.ScriptTarget.ES2020,
      esModuleInterop: true,
    },
    fileName: profilePath,
  });
  const loader = Module as unknown as {
    _load: (request: string, parent: unknown, isMain: boolean) => unknown;
    _nodeModulePaths: (from: string) => string[];
  };
  const originalLoad = loader._load;
  const host = (tag: string) => ({
    children,
    className,
    style,
  }: {
    children?: ReactNode;
    className?: string;
    style?: {backgroundColor?: string; color?: string};
  }) =>
    React.createElement(
      tag,
      {
        ...(className ? {'data-layout': className} : {}),
        ...(style?.backgroundColor ? {'data-background-color': style.backgroundColor} : {}),
        ...(style?.color ? {'data-color': style.color} : {}),
      },
      children,
    );
  const profileModule = new Module(profilePath) as Module & {
    _compile: (content: string, filename: string) => void;
  };
  profileModule.filename = profilePath;
  profileModule.paths = loader._nodeModulePaths(dirname(profilePath));

  loader._load = (request, parent, isMain) => {
    switch (request) {
      case '@expo/vector-icons':
        return {Ionicons: () => null};
      case '@tanstack/react-query':
        return originalLoad(request, parent, isMain);
      case 'expo-router':
        return {useRouter: () => ({push: () => undefined})};
      case 'react-native':
        return {
          Modal: ({children, visible}: {children?: ReactNode; visible?: boolean}) =>
            visible ? React.createElement('div', null, children) : null,
          Pressable: host('button'),
          Text: host('span'),
          View: host('div'),
          useWindowDimensions: () => ({width: 390}),
        };
      case '@/components/ConfirmModal':
        return {ConfirmModal: () => null};
      case '@/components/Screen':
        return {Screen: host('main')};
      case '@/i18n/useI18n':
        return {
          useI18n: () => ({
            language,
            t: (key: string) =>
              translations[key] ??
              ({'profile.free': '免费版', 'profile.subscription': '订阅版'})[key] ??
              key,
          }),
        };
      case '@/i18n/languages':
        return {languageLabelMap: {'zh-Hans': '简体中文'}};
      case '@/lib/api':
        return {api: apiClient};
      case '@/lib/fluxaBalance':
        return {formatFluxABalance};
      case '@/store/auth':
        return {
          useAuthStore: (selector: (state: Record<string, unknown>) => unknown) =>
            selector({
              user: {
                id: 'user-1',
                displayName: 'FluxA User',
                username: 'fluxa-user',
                fluxaSite: 'paid',
                fluxaGroup: '默认分组',
              },
              accessToken: 'knowvia-access-token',
              logout: () => undefined,
            }),
        };
      case '@/store/preferences':
        return {
          usePreferencesStore: (selector: (state: Record<string, unknown>) => unknown) =>
            selector({appearance: 'light', setAppearance: () => undefined}),
        };
      case '@/theme/typography':
        return {fontSizes: {xs: 12, sm: 14, md: 16, lg: 18, xl: 20}};
      case '@/theme/useAppTheme':
        return {
          useAppTheme: () => ({
            colors,
          }),
        };
      default:
        return originalLoad(request, parent, isMain);
    }
  };

  try {
    profileModule._compile(compiled.outputText, profilePath);
    return {
      ProfileScreen: profileModule.exports.default as ComponentType,
      resolveFluxAPlan: profileModule.exports.resolveFluxAPlan as
        | ((group: string) => {titleKey: string; badgeLabel?: string} | undefined)
        | undefined,
    };
  } finally {
    loader._load = originalLoad;
  }
}

function contrastRatio(foreground: string, background: string) {
  const luminance = (color: string) => {
    const hex = color.slice(1);
    const rgb = [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255);
    const channels = rgb.map((channel) =>
      channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4,
    );

    return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
  };
  const [lighter, darker] = [luminance(foreground), luminance(background)].sort((a, b) => b - a);

  return (lighter + 0.05) / (darker + 0.05);
}

function loadApiForTest(): Pick<typeof ApiClient, 'getFluxABalance' | 'listFluxAModelGroups' | 'listFluxAModels'> {
  const loader = Module as unknown as {
    _load: (request: string, parent: unknown, isMain: boolean) => unknown;
  };
  const originalLoad = loader._load;
  loader._load = (request, parent, isMain) => {
    switch (request) {
      case 'expo-constants':
        return {__esModule: true, default: {expoConfig: {}, linkingUri: '', experienceUrl: '', intentUri: ''}};
      case 'react-native':
        return {Platform: {OS: 'ios'}};
      case 'react-native-sse':
        return {__esModule: true, default: class EventSource {}};
      case '@/lib/apiRequest':
        return originalLoad('../lib/apiRequest', parent, isMain);
      case '@/lib/fluxaApi':
        return originalLoad('../lib/fluxaApi', parent, isMain);
      case '@/store/auth':
        return {useAuthStore: () => null};
      default:
        return originalLoad(request, parent, isMain);
    }
  };

  try {
    return require('../lib/api').api;
  } finally {
    loader._load = originalLoad;
  }
}

test('FluxA 2FA errors from the safe backend response propagate to the UI', async () => {
  await assert.rejects(
    loginThroughFluxA(
      {site: 'paid', username: 'user', password: 'secret'},
      {
        loginWithFluxA: async () => {
          throw new FluxA2FARequiredError();
        },
        persistSession: async () => undefined,
      },
    ),
    (error: unknown) => error instanceof FluxA2FARequiredError,
  );
});
