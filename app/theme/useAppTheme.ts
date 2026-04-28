import {useColorScheme} from 'react-native';

import {usePreferencesStore} from '@/store/preferences';
import {appThemes} from '@/theme/colors';
import type {ThemeName} from '@/theme/colors';

export function useAppTheme() {
  const appearance = usePreferencesStore((state) => state.appearance);
  const systemColorScheme = useColorScheme();

  const themeName: ThemeName =
    appearance === 'system'
      ? systemColorScheme === 'dark'
        ? 'dark'
        : 'light'
      : appearance;

  const colors = appThemes[themeName];

  return {
    appearance,
    colors,
    themeName,
    isDark: themeName === 'dark',
    statusBarStyle: colors.statusBarStyle,
  };
}
