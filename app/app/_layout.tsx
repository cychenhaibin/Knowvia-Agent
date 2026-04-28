import {Stack} from 'expo-router';
import {ActivityIndicator, View} from 'react-native';
import {QueryClientProvider} from '@tanstack/react-query';
import {StatusBar} from 'expo-status-bar';
import {useEffect} from 'react';
import {SafeAreaProvider, SafeAreaView} from 'react-native-safe-area-context';

import '@/global.css';

import {queryClient} from '@/lib/query-client';
import {useAuthStore} from '@/store/auth';
import {usePreferencesStore} from '@/store/preferences';
import {useSkillsStore} from '@/store/skills';
import {useAppTheme} from '@/theme/useAppTheme';

export default function RootLayout() {
  const bootstrap = useAuthStore((state) => state.bootstrap);
  const bootstrapped = useAuthStore((state) => state.bootstrapped);
  const accessToken = useAuthStore((state) => state.accessToken);
  const bootstrapPreferences = usePreferencesStore((state) => state.bootstrap);
  const preferencesBootstrapped = usePreferencesStore((state) => state.bootstrapped);
  const bootstrapSkills = useSkillsStore((state) => state.bootstrap);
  const refreshSkills = useSkillsStore((state) => state.refresh);
  const skillsBootstrapped = useSkillsStore((state) => state.bootstrapped);
  const {colors, statusBarStyle} = useAppTheme();

  useEffect(() => {
    void bootstrap();
    void bootstrapPreferences();
    void bootstrapSkills();
  }, [bootstrap, bootstrapPreferences, bootstrapSkills]);

  useEffect(() => {
    if (!bootstrapped || !skillsBootstrapped) {
      return;
    }
    void refreshSkills();
  }, [accessToken, bootstrapped, refreshSkills, skillsBootstrapped]);

  return (
    <SafeAreaProvider>
      <QueryClientProvider client={queryClient}>
        <StatusBar style={statusBarStyle} />
        {bootstrapped && preferencesBootstrapped && skillsBootstrapped ? (
          <Stack screenOptions={{headerShown: false}}>
            <Stack.Screen name="index" />
            <Stack.Screen name="(auth)" />
            <Stack.Screen name="(tabs)" />
            <Stack.Screen
              name="(modals)/compose"
              options={{presentation: 'modal', headerShown: false}}
            />
            <Stack.Screen
              name="(modals)/run-history"
              options={{presentation: 'modal', headerShown: false}}
            />
            <Stack.Screen
              name="(modals)/upgrade"
              options={{presentation: 'modal', headerShown: false}}
            />
            <Stack.Screen name="(account)/language" options={{headerShown: false}} />
            <Stack.Screen name="(account)/models" options={{headerShown: false}} />
            <Stack.Screen
              name="(account)/profile"
              options={{headerShown: false, animation: 'slide_from_right'}}
            />
            <Stack.Screen
              name="knowledge/index"
              options={{headerShown: false, animation: 'slide_from_right'}}
            />
            <Stack.Screen name="knowledge/[id]" options={{headerShown: false}} />
            <Stack.Screen name="(skills)/skill/[id]" options={{headerShown: false}} />
            <Stack.Screen name="(skills)/skills" options={{headerShown: false}} />
            <Stack.Screen name="(account)/usage" options={{headerShown: false}} />
            <Stack.Screen name="runs/[id]/index" />
          </Stack>
        ) : (
          <SafeAreaView
            className="flex-1"
            style={{backgroundColor: colors.background}}
            edges={['top', 'bottom', 'left', 'right']}>
            <View
              className="flex-1 items-center justify-center"
              style={{backgroundColor: colors.background}}>
              <ActivityIndicator size="large" color={colors.brand} />
            </View>
          </SafeAreaView>
        )}
      </QueryClientProvider>
    </SafeAreaProvider>
  );
}
