import {Ionicons} from '@expo/vector-icons';
import {useRouter} from 'expo-router';
import {Pressable, ScrollView, Text, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {useI18n} from '@/i18n/useI18n';
import {languageOptions} from '@/i18n/languages';
import {usePreferencesStore} from '@/store/preferences';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

export default function LanguageScreen() {
  const router = useRouter();
  const language = usePreferencesStore((state) => state.language);
  const setLanguage = usePreferencesStore((state) => state.setLanguage);
  const {colors} = useAppTheme();
  const {t} = useI18n();

  return (
    <SafeAreaView
      className="flex-1"
      style={{backgroundColor: colors.background}}
      edges={['top', 'bottom', 'left', 'right']}>
      <View
        className="mb-2 flex-row items-center justify-between px-3 pt-3"
        style={{backgroundColor: colors.background}}>
        <Pressable
          className="h-11 w-11 items-center justify-center rounded-full"
          onPress={() => router.back()}>
          <Ionicons name="chevron-back" size={22} color={colors.textPrimary} />
        </Pressable>
        <Text
          style={{fontSize: fontSizes.lg, fontWeight: '500', color: colors.textPrimary}}>
          {t('language.title')}
        </Text>
        <View className="w-11" />
      </View>

      <ScrollView
        className="flex-1"
        contentContainerStyle={{paddingHorizontal: 24, paddingBottom: 24}}
        showsVerticalScrollIndicator={false}>
        <View className="gap-8">
          {languageOptions.map((option) => {
            const selected = option.key === language;

            return (
              <Pressable
                key={option.key}
                className="flex-row items-center justify-between"
                onPress={() => {
                  void setLanguage(option.key);
                }}>
                <View className="flex-1 pr-4">
                  <Text
                    style={{
                      fontSize: fontSizes.md,
                      fontWeight: '500',
                      color: colors.textPrimary,
                    }}>
                    {option.label}
                  </Text>
                  <Text
                    style={{
                      fontSize: fontSizes.xs,
                      color: colors.textMuted,
                    }}>
                    {option.subtitle}
                  </Text>
                </View>
                {selected ? (
                  <Ionicons name="checkmark" size={26} color={colors.brand} />
                ) : null}
              </Pressable>
            );
          })}
        </View>
      </ScrollView>
    </SafeAreaView>
  );
}
