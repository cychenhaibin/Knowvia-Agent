import type {SessionPayload} from '@/types/api';

export type FluxASite = 'paid' | 'free';

export const FLUXA_2FA_REQUIRED_BACKEND_MESSAGE =
  'complete two-factor authentication on the selected FluxA site';

export class FluxA2FARequiredError extends Error {
  constructor() {
    super('FluxA login requires completing two-factor authentication on the selected site.');
    this.name = 'FluxA2FARequiredError';
  }
}

export type FluxALoginDependencies = {
  loginWithFluxA: (
    site: FluxASite,
    username: string,
    password: string,
  ) => Promise<SessionPayload>;
  persistSession: (session: SessionPayload) => Promise<void>;
};

export function canSubmit(
  site: FluxASite | null,
  username: string,
  password: string,
  pending: boolean,
): boolean {
  return Boolean(site && username.trim() && password.trim() && !pending);
}

export function credentialsAreEditable(site: FluxASite | null): boolean {
  return site !== null;
}

export function nextPasswordAfterSiteChange(
  previousSite: FluxASite | null,
  nextSite: FluxASite | null,
): string | null {
  return previousSite === nextSite ? null : '';
}

export async function loginThroughFluxA(
  input: {site: FluxASite; username: string; password: string},
  dependencies: FluxALoginDependencies,
): Promise<SessionPayload> {
  const session = await dependencies.loginWithFluxA(
    input.site,
    input.username.trim(),
    input.password.trim(),
  );
  await dependencies.persistSession(session);
  return session;
}
