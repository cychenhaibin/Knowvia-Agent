import * as SecureStore from 'expo-secure-store';
import {create} from 'zustand';

import type {AppLanguage} from '@/i18n/languages';
import {isAppLanguage} from '@/i18n/languages';
import type {AppearanceMode} from '@/theme/colors';

const APPEARANCE_KEY = 'quickque-agent-appearance';
const LANGUAGE_KEY = 'quickque-agent-language';

type PreferencesState = {
  appearance: AppearanceMode;
  language: AppLanguage;
  bootstrapped: boolean;
  bootstrap: () => Promise<void>;
  setAppearance: (appearance: AppearanceMode) => Promise<void>;
  setLanguage: (language: AppLanguage) => Promise<void>;
};

function isAppearanceMode(value: string | null): value is AppearanceMode {
  return value === 'system' || value === 'light' || value === 'dark';
}

export const usePreferencesStore = create<PreferencesState>((set) => ({
  appearance: 'system',
  language: 'zh-Hans',
  bootstrapped: false,
  bootstrap: async () => {
    const [appearance, language] = await Promise.all([
      SecureStore.getItemAsync(APPEARANCE_KEY),
      SecureStore.getItemAsync(LANGUAGE_KEY),
    ]);

    set({
      appearance: isAppearanceMode(appearance) ? appearance : 'system',
      language: isAppLanguage(language) ? language : 'zh-Hans',
      bootstrapped: true,
    });
  },
  setAppearance: async (appearance) => {
    await SecureStore.setItemAsync(APPEARANCE_KEY, appearance);
    set({appearance});
  },
  setLanguage: async (language) => {
    await SecureStore.setItemAsync(LANGUAGE_KEY, language);
    set({language});
  },
}));
