import {create} from 'zustand';

import {clearGoogleCredentialState} from '@/lib/google-auth';
import {clearMicrosoftAccountState} from '@/lib/microsoft-auth';
import {clearSession, loadSession, saveSession} from '@/lib/session';
import type {SessionPayload, User} from '@/types/api';

type AuthState = {
  bootstrapped: boolean;
  user: User | null;
  accessToken: string | null;
  refreshToken: string | null;
  bootstrap: () => Promise<void>;
  setSession: (payload: SessionPayload) => Promise<void>;
  logout: () => Promise<void>;
};

export const useAuthStore = create<AuthState>((set) => ({
  bootstrapped: false,
  user: null,
  accessToken: null,
  refreshToken: null,
  bootstrap: async () => {
    const session = await loadSession();
    if (!session) {
      set({bootstrapped: true});
      return;
    }

    set({
      bootstrapped: true,
      accessToken: session.accessToken,
      refreshToken: session.refreshToken,
      user: JSON.parse(session.user),
    });
  },
  setSession: async (payload) => {
    await saveSession({
      accessToken: payload.accessToken,
      refreshToken: payload.refreshToken,
      user: JSON.stringify(payload.user),
    });
    set({
      user: payload.user,
      accessToken: payload.accessToken,
      refreshToken: payload.refreshToken,
      bootstrapped: true,
    });
  },
  logout: async () => {
    await clearSession();
    await clearGoogleCredentialState();
    await clearMicrosoftAccountState();
    set({
      user: null,
      accessToken: null,
      refreshToken: null,
      bootstrapped: true,
    });
  },
}));
