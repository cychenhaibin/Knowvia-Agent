import {Tabs} from 'expo-router';

import {useI18n} from '@/i18n/useI18n';

export default function TabsLayout() {
  const {t} = useI18n();

  return (
    <Tabs
      screenOptions={{
        headerShown: false,
      }}
      tabBar={() => null}>
      <Tabs.Screen
        name="runs"
        options={{
          title: t('tab.runs'),
        }}
      />
    </Tabs>
  );
}
