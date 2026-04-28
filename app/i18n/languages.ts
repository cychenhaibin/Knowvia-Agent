export type AppLanguage =
  | 'en'
  | 'zh-Hans'
  | 'zh-Hant'
  | 'ja'
  | 'ko'
  | 'de'
  | 'es-ES'
  | 'es-419'
  | 'ar'
  | 'fr'
  | 'id'
  | 'it'
  | 'pt-PT'
  | 'pt-BR'
  | 'tr'
  | 'vi'
  | 'th';

export type LanguageOption = {
  key: AppLanguage;
  label: string;
  subtitle: string;
};

export const languageOptions: LanguageOption[] = [
  {key: 'en', label: 'English', subtitle: '英语'},
  {key: 'zh-Hans', label: '简体中文', subtitle: '中文（简体）'},
  {key: 'zh-Hant', label: '繁體中文', subtitle: '中文（繁體）'},
  {key: 'ja', label: '日本語', subtitle: '日语'},
  {key: 'ko', label: '한국어', subtitle: '韩语'},
  {key: 'de', label: 'Deutsch', subtitle: '德语'},
  {key: 'es-ES', label: 'Español (España)', subtitle: '西班牙语（西班牙）'},
  {
    key: 'es-419',
    label: 'Español (Latinoamérica)',
    subtitle: '西班牙语（拉丁美洲）',
  },
  {key: 'ar', label: 'العربية', subtitle: '阿拉伯语'},
  {key: 'fr', label: 'Français', subtitle: '法语'},
  {key: 'id', label: 'Bahasa Indonesia', subtitle: '印度尼西亚语'},
  {key: 'it', label: 'Italiano', subtitle: '意大利语'},
  {key: 'pt-PT', label: 'Português (Portugal)', subtitle: '葡萄牙语（葡萄牙）'},
  {key: 'pt-BR', label: 'Português (Brasil)', subtitle: '葡萄牙语（巴西）'},
  {key: 'tr', label: 'Türkçe', subtitle: '土耳其语'},
  {key: 'vi', label: 'Tiếng Việt', subtitle: '越南语'},
  {key: 'th', label: 'ไทย', subtitle: '泰语'},
];

export const languageLabelMap: Record<AppLanguage, string> = Object.fromEntries(
  languageOptions.map((option) => [option.key, option.label]),
) as Record<AppLanguage, string>;

export function isAppLanguage(value: string | null): value is AppLanguage {
  return languageOptions.some((option) => option.key === value);
}
