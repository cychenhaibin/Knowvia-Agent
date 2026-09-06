import {requestFromSingleBase, type FetchLike} from './apiRequest';
import {
  FluxA2FARequiredError,
  FLUXA_2FA_REQUIRED_BACKEND_MESSAGE,
} from '../modules/auth/fluxaFlow';
import type {FluxASite, SessionPayload} from '../types/api';

export function createFluxALogin(baseUrl: string, fetchImpl: FetchLike = fetch) {
  return async (
    site: FluxASite,
    username: string,
    password: string,
  ): Promise<SessionPayload> => {
    try {
      return await requestFromSingleBase<SessionPayload>(
        baseUrl,
        '/auth/fluxa',
        {
          method: 'POST',
          body: JSON.stringify({site, username, password}),
        },
        undefined,
        fetchImpl,
      );
    } catch (error) {
      if (error instanceof Error && error.message === FLUXA_2FA_REQUIRED_BACKEND_MESSAGE) {
        throw new FluxA2FARequiredError();
      }
      throw error;
    }
  };
}
