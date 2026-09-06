import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

import type {signInWithGoogle} from '../lib/google-auth';

import {getDictionary} from '../i18n/messages';
import type {FluxASite, SessionPayload} from '../types/api';
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
          user: {id: '1', username: 'fluxa-free-42', displayName: 'FluxA User'},
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
  assert.deepEqual(result.user, {id: '1', username: 'fluxa-free-42', displayName: 'FluxA User'});
  assert.deepEqual(saved, [result]);
});

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
