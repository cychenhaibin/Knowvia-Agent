export type FluxASite = 'paid' | 'free';

export const FLUXA_SITE_ORIGINS: Record<FluxASite, string> = {
  paid: 'https://fluxa.camila.qzz.io',
  free: 'https://free.camila.qzz.io',
};

export function resolveFluxASiteOrigin(site: unknown): string {
  if (site === 'paid' || site === 'free') {
    return FLUXA_SITE_ORIGINS[site];
  }
  throw new Error('Unsupported FluxA site');
}

export type FluxALoginResponse = {
  accessToken?: string;
  require2FA?: boolean;
};

export type FluxASession = {
  user: {id: string; username: string; displayName: string; email?: string; avatarUrl?: string};
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
};

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
  ) => Promise<FluxALoginResponse>;
  exchangeFluxASession: (site: FluxASite, accessToken: string) => Promise<FluxASession>;
  persistSession: (session: FluxASession) => Promise<void>;
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
): Promise<FluxASession> {
  const login = await dependencies.loginWithFluxA(
    input.site,
    input.username.trim(),
    input.password,
  );
  if (login.require2FA) {
    throw new FluxA2FARequiredError();
  }
  if (!login.accessToken) {
    throw new Error('FluxA verification failed');
  }

  const session = await dependencies.exchangeFluxASession(input.site, login.accessToken);
  await dependencies.persistSession(session);
  return session;
}
