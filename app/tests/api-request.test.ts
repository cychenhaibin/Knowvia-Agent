import assert from 'node:assert/strict';
import test from 'node:test';

import {
  requestFromCandidates,
  requestFromSingleBase,
  type FetchLike,
} from '../lib/apiRequest';
import {createFluxALogin} from '../lib/fluxaApi';
import {FluxA2FARequiredError} from '../modules/auth/fluxaFlow';

test('credential requests never replay a POST to fallback origins', async () => {
  const calls: string[] = [];
  const fetchImpl: FetchLike = async (input) => {
    calls.push(String(input));
    throw new Error('offline');
  };

  await assert.rejects(
    requestFromSingleBase(
      'https://primary.example/v1',
      '/auth/fluxa',
      {method: 'POST', body: '{"password":" secret "}'},
      undefined,
      fetchImpl,
    ),
    /Network error: offline/,
  );
  assert.deepEqual(calls, ['https://primary.example/v1/auth/fluxa']);
});

test('ordinary requests retain candidate fallback after a network failure', async () => {
  const calls: string[] = [];
  const fetchImpl: FetchLike = async (input) => {
    const url = String(input);
    calls.push(url);
    if (url.startsWith('https://primary.example')) {
      throw new Error('primary offline');
    }
    return new Response('{"status":"ok"}', {
      status: 200,
      headers: {'Content-Type': 'application/json'},
    });
  };

  const result = await requestFromCandidates<{status: string}>(
    ['https://primary.example/v1', 'https://fallback.example/v1'],
    '/health',
    {},
    undefined,
    fetchImpl,
  );

  assert.deepEqual(calls, [
    'https://primary.example/v1/health',
    'https://fallback.example/v1/health',
  ]);
  assert.deepEqual(result, {status: 'ok'});
});

test('production FluxA API posts exact credentials once and returns the backend session', async () => {
  const calls: Array<{url: string; body: string}> = [];
  const fetchImpl: FetchLike = async (input, init) => {
    calls.push({url: String(input), body: String(init?.body)});
    return new Response(JSON.stringify({
      accessToken: 'knowvia-access',
      refreshToken: 'knowvia-refresh',
      expiresIn: 3600,
      user: {id: '1', username: 'fluxa-paid-42', displayName: 'FluxA User'},
    }), {
      status: 200,
      headers: {'Content-Type': 'application/json'},
    });
  };

  const loginWithFluxA = createFluxALogin('https://primary.example/v1', fetchImpl);
  const session = await loginWithFluxA('paid', 'trimmed-user', ' password\t ');

  assert.deepEqual(calls, [{
    url: 'https://primary.example/v1/auth/fluxa',
    body: '{"site":"paid","username":"trimmed-user","password":" password\\t "}',
  }]);
  assert.equal(session.accessToken, 'knowvia-access');
  assert.equal(session.user.username, 'fluxa-paid-42');
});

test('production FluxA API maps a non-200 backend 2FA response for the UI', async () => {
  const fetchImpl: FetchLike = async () => new Response(JSON.stringify({
    code: 'unauthorized',
    error: 'complete two-factor authentication on the selected FluxA site',
  }), {
    status: 401,
    headers: {'Content-Type': 'application/json'},
  });

  const loginWithFluxA = createFluxALogin('https://primary.example/v1', fetchImpl);
  await assert.rejects(
    loginWithFluxA('free', 'user', 'password'),
    (error: unknown) => error instanceof FluxA2FARequiredError,
  );
});
