import type {PropsWithChildren, Ref} from 'react';
import {ScrollView, View} from 'react-native';
import {SafeAreaView} from 'react-native-safe-area-context';

import {useAppTheme} from '@/theme/useAppTheme';

type ScreenProps = PropsWithChildren<{
  scroll?: boolean;
  scrollViewRef?: Ref<ScrollView>;
}>;

export function Screen({children, scroll = false, scrollViewRef}: ScreenProps) {
  const {colors} = useAppTheme();

  const content = (
    <View className="flex-1 px-5 py-4" style={{backgroundColor: colors.background}}>
      {children}
    </View>
  );

  if (scroll) {
    return (
      <SafeAreaView
        className="flex-1"
        style={{backgroundColor: colors.background}}
        edges={['top', 'bottom', 'left', 'right']}>
        <ScrollView
          ref={scrollViewRef}
          contentContainerStyle={{flexGrow: 1}}
          showsVerticalScrollIndicator={false}>
          {content}
        </ScrollView>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView
      className="flex-1"
      style={{backgroundColor: colors.background}}
      edges={['top', 'bottom', 'left', 'right']}>
      {content}
    </SafeAreaView>
  );
}
