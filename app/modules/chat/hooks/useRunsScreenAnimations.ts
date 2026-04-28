import {useEffect, useRef} from 'react';
import {Animated, Easing} from 'react-native';

import {
  DRAWER_CLOSE_DURATION,
  DRAWER_OPEN_DURATION,
  PROFILE_NAV_EXIT_DURATION,
} from '@/modules/chat/constants';

export function useRunsScreenAnimations({
  drawerWidth,
  showHistoryDrawer,
}: {
  drawerWidth: number;
  showHistoryDrawer: boolean;
}) {
  const drawerProgress = useRef(new Animated.Value(0)).current;
  const profileExitProgress = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    drawerProgress.stopAnimation();
    Animated.timing(drawerProgress, {
      toValue: showHistoryDrawer ? 1 : 0,
      duration: showHistoryDrawer ? DRAWER_OPEN_DURATION : DRAWER_CLOSE_DURATION,
      easing: Easing.out(Easing.cubic),
      useNativeDriver: true,
    }).start();
  }, [drawerProgress, showHistoryDrawer]);

  const contentTranslateX = drawerProgress.interpolate({
    inputRange: [0, 1],
    outputRange: [0, drawerWidth],
  });
  const drawerTranslateX = drawerProgress.interpolate({
    inputRange: [0, 1],
    outputRange: [-drawerWidth * 0.14, 0],
  });
  const contentOverlayOpacity = drawerProgress.interpolate({
    inputRange: [0, 1],
    outputRange: [0, 1],
  });
  const contentExitOpacity = profileExitProgress.interpolate({
    inputRange: [0, 1],
    outputRange: [1, 0.9],
  });
  const contentExitScale = profileExitProgress.interpolate({
    inputRange: [0, 1],
    outputRange: [1, 0.985],
  });
  const contentExitTranslateY = profileExitProgress.interpolate({
    inputRange: [0, 1],
    outputRange: [0, -8],
  });

  const runProfileExitAnimation = () =>
    new Promise<boolean>((resolve) => {
      profileExitProgress.stopAnimation();
      profileExitProgress.setValue(0);
      Animated.timing(profileExitProgress, {
        toValue: 1,
        duration: PROFILE_NAV_EXIT_DURATION,
        easing: Easing.out(Easing.cubic),
        useNativeDriver: true,
      }).start(({finished}) => {
        resolve(finished);
      });
    });

  const resetProfileExitAnimation = () => {
    profileExitProgress.setValue(0);
  };

  return {
    contentTranslateX,
    drawerTranslateX,
    contentOverlayOpacity,
    contentExitOpacity,
    contentExitScale,
    contentExitTranslateY,
    runProfileExitAnimation,
    resetProfileExitAnimation,
  };
}
