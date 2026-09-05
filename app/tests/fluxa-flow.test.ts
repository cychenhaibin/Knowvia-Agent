import assert from 'node:assert/strict';
import test from 'node:test';

import {getDictionary} from '../i18n/messages';
import {
  FluxA2FARequiredError,
  loginThroughFluxA,
  nextPasswordAfterSiteChange,
  canSubmit,
  credentialsAreEditable,
  resolveFluxASiteOrigin,
} from '../modules/auth/fluxaFlow';

test('Chinese FluxA labels identify the service as the transit station', () => {
  const messages = getDictionary('zh-Hans');
  assert.equal(messages['login.fluxaLogin'], 'FluxA中转站登录');
  assert.equal(messages['login.fluxaTitle'], 'FluxA中转站登录');
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

test('paid and free logins resolve to their independent FluxA origins', () => {
  assert.equal(resolveFluxASiteOrigin('paid'), 'https://fluxa.camila.qzz.io');
  assert.equal(resolveFluxASiteOrigin('free'), 'https://free.camila.qzz.io');
  assert.throws(() => resolveFluxASiteOrigin('https://attacker.example'), /Unsupported FluxA site/);
});

test('FluxA login routes paid and free origins, exchanges the upstream token, and persists only the Knowvia session', async () => {
  const calls: string[] = [];
  const saved: unknown[] = [];
  const result = await loginThroughFluxA(
    {site: 'free', username: ' user ', password: 'secret'},
    {
      loginWithFluxA: async (site, username, password) => {
        calls.push(`login:${site}:${username}:${password}`);
        return {accessToken: 'upstream-token'};
      },
      exchangeFluxASession: async (site, accessToken) => {
        calls.push(`exchange:${site}:${accessToken}`);
        return {
          accessToken: 'knowvia-access',
          refreshToken: 'knowvia-refresh',
          expiresIn: 3600,
          user: {id: '1', username: 'fluxa-free-42', displayName: 'FluxA User'},
        };
      },
      persistSession: async (session) => {
        saved.push(session);
      },
    },
  );

  assert.deepEqual(calls, [
    'login:free:user:secret',
    'exchange:free:upstream-token',
  ]);
  assert.deepEqual(result.user, {id: '1', username: 'fluxa-free-42', displayName: 'FluxA User'});
  assert.deepEqual(saved, [result]);
  assert.equal(JSON.stringify(saved).includes('upstream-token'), false);
});

test('FluxA 2FA responses produce a user-facing typed error before exchange', async () => {
  let exchanged = false;
  await assert.rejects(
    loginThroughFluxA(
      {site: 'paid', username: 'user', password: 'secret'},
      {
        loginWithFluxA: async () => ({require2FA: true}),
        exchangeFluxASession: async () => {
          exchanged = true;
          throw new Error('must not exchange');
        },
        persistSession: async () => undefined,
      },
    ),
    (error: unknown) => error instanceof FluxA2FARequiredError,
  );
  assert.equal(exchanged, false);
});
