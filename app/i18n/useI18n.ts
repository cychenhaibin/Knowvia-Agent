import {useMemo} from 'react';

import {usePreferencesStore} from '@/store/preferences';
import {getDictionary, type MessageKey} from '@/i18n/messages';
import {enMessages} from '@/i18n/messages';
import type {RunMode, RunStatus, StepStatus} from '@/types/api';

type Params = Record<string, string | number>;

function interpolate(template: string, params?: Params) {
  if (!params) {
    return template;
  }

  return Object.entries(params).reduce(
    (result, [key, value]) => result.replaceAll(`{${key}}`, String(value)),
    template,
  );
}

export function useI18n() {
  const language = usePreferencesStore((state) => state.language);

  const dictionary = useMemo(() => getDictionary(language), [language]);

  const t = (key: MessageKey, params?: Params) => {
    const template = dictionary[key] ?? enMessages[key] ?? String(key);
    return interpolate(template, params);
  };

  const modeLabel = (mode: RunMode) => t(`mode.${mode}` as MessageKey);
  const statusLabel = (status: RunStatus | StepStatus) =>
    t(`status.${status}` as MessageKey);

  return {
    language,
    t,
    modeLabel,
    statusLabel,
  };
}
