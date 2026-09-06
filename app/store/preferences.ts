import * as SecureStore from 'expo-secure-store';
import {create} from 'zustand';

import type {AppLanguage} from '@/i18n/languages';
import {isAppLanguage} from '@/i18n/languages';
import type {AppearanceMode} from '@/theme/colors';

const APPEARANCE_KEY = 'quickque-agent-appearance';
const LANGUAGE_KEY = 'quickque-agent-language';
const FLUXA_MODELS_KEY = 'quickque-agent-fluxa-models';

export type EnabledFluxAModel = {id: string; name: string; group: string};

type PreferencesState = {
  appearance: AppearanceMode;
  language: AppLanguage;
  enabledFluxAModelsByUser: Record<string, EnabledFluxAModel[]>;
  bootstrapped: boolean;
  bootstrap: () => Promise<void>;
  setAppearance: (appearance: AppearanceMode) => Promise<void>;
  setLanguage: (language: AppLanguage) => Promise<void>;
  setEnabledFluxAModels: (userId: string, models: EnabledFluxAModel[]) => Promise<void>;
};

function isAppearanceMode(value: string | null): value is AppearanceMode {
  return value === 'system' || value === 'light' || value === 'dark';
}

export const usePreferencesStore = create<PreferencesState>((set) => ({
  appearance: 'system',
  language: 'zh-Hans',
  enabledFluxAModelsByUser: {},
  bootstrapped: false,
  bootstrap: async () => {
    const [appearance, language, fluxaModels] = await Promise.all([
      SecureStore.getItemAsync(APPEARANCE_KEY),
      SecureStore.getItemAsync(LANGUAGE_KEY),
      SecureStore.getItemAsync(FLUXA_MODELS_KEY),
    ]);

    let enabledFluxAModelsByUser: Record<string, EnabledFluxAModel[]> = {};
    try {
      const parsed: unknown = fluxaModels ? JSON.parse(fluxaModels) : {};
      if (parsed && typeof parsed === 'object') enabledFluxAModelsByUser = parsed as Record<string, EnabledFluxAModel[]>;
    } catch {}

    set({
      appearance: isAppearanceMode(appearance) ? appearance : 'system',
      language: isAppLanguage(language) ? language : 'zh-Hans',
      enabledFluxAModelsByUser,
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
  setEnabledFluxAModels: async (userId, models) => {
    const current = usePreferencesStore.getState().enabledFluxAModelsByUser;
    const next = {...current, [userId]: models};
    await SecureStore.setItemAsync(FLUXA_MODELS_KEY, JSON.stringify(next));
    set({enabledFluxAModelsByUser: next});
  },
}));
