export type AppearanceMode = 'system' | 'light' | 'dark';
export type ThemeName = 'light' | 'dark';

export const appearanceLabelMap: Record<AppearanceMode, string> = {
  system: '跟随系统',
  light: '浅色模式',
  dark: '深色模式',
};

export const appThemes = {
  light: {
    brand: '#f24f2d',
    brandSoft: 'rgba(242, 79, 45, 0.12)',
    background: '#f7f7f5',
    surface: '#ffffff',
    surfaceMuted: '#f0f1f3',
    surfaceOverlay: 'rgba(255,255,255,0.9)',
    closeButtonBg: 'rgba(255,255,255,0.8)',
    textPrimary: '#171717',
    textSecondary: '#6b7280',
    textMuted: '#8f959e',
    textTertiary: '#9ca3af',
    icon: '#2f2f33',
    iconSubtle: '#a3a7ad',
    border: '#eceef1',
    divider: '#f1f3f5',
    overlay: 'rgba(23,23,23,0.45)',
    overlaySoft: 'rgba(23,23,23,0.08)',
    pressed: 'rgba(0,0,0,0.03)',
    pressedStrong: 'rgba(0,0,0,0.04)',
    shadow: '#171717',
    statusBarStyle: 'dark' as const,
  },
  dark: {
    brand: '#f24f2d',
    brandSoft: 'rgba(242, 79, 45, 0.2)',
    background: '#1c1c1c',
    surface: '#2a2a2a',
    surfaceMuted: '#3a3a3a',
    surfaceOverlay: 'rgba(42,42,42,0.94)',
    closeButtonBg: 'rgba(42,42,42,0.92)',
    textPrimary: '#f7f7f7',
    textSecondary: '#d0d0d0',
    textMuted: '#9f9f9f',
    textTertiary: '#7d7d7d',
    icon: '#f3f3f3',
    iconSubtle: '#8e8e8e',
    border: '#343434',
    divider: '#353535',
    overlay: 'rgba(0,0,0,0.62)',
    overlaySoft: 'rgba(0,0,0,0.34)',
    pressed: 'rgba(255,255,255,0.06)',
    pressedStrong: 'rgba(255,255,255,0.1)',
    shadow: '#000000',
    statusBarStyle: 'light' as const,
  },
} as const;

export type AppColors = (typeof appThemes)[ThemeName];

export const appColors = appThemes.light;
