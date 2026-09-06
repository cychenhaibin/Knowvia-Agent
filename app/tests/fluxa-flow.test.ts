import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import Module from 'node:module';
import test from 'node:test';

import {getDictionary} from '../i18n/messages';
import type {api as ApiClient} from '../lib/api';
import type {FluxASite, SessionPayload} from '../types/api';
import {
  FluxA2FARequiredError,
  loginThroughFluxA,
  nextPasswordAfterSiteChange,
  canSubmit,
  credentialsAreEditable,
} from '../modules/auth/fluxaFlow';

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
  assert.equal(fetchCall.url.endsWith('/fluxa/model-groups'), true);
  assert.equal(fetchCall.headers.get('Authorization'), 'Bearer knowvia-token');
});

function loadApiForTest(): Pick<typeof ApiClient, 'listFluxAModelGroups'> {
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
