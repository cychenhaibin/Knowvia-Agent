import {useEffect, useMemo, useRef, useState, type ReactNode} from 'react';
import {
  Animated,
  Easing,
  Modal,
  PanResponder,
  Pressable,
  Text,
  View,
  useWindowDimensions,
} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';

import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

type BottomSheetProps = {
  visible: boolean;
  onClose: () => void;
  title?: string;
  headerRight?: ReactNode;
  children: ReactNode;
};

const HIDDEN_OFFSET = 520;

export function BottomSheet({
  visible,
  onClose,
  title,
  headerRight,
  children,
}: BottomSheetProps) {
  const {colors} = useAppTheme();
  const insets = useSafeAreaInsets();
  const {height: windowHeight} = useWindowDimensions();
  const [mounted, setMounted] = useState(visible);
  const translateY = useRef(new Animated.Value(HIDDEN_OFFSET)).current;
  const backdropOpacity = useRef(new Animated.Value(0)).current;
  const sheetHeightRef = useRef(HIDDEN_OFFSET);
  const currentTranslateY = useRef(HIDDEN_OFFSET);

  useEffect(() => {
    const id = translateY.addListener(({value}) => {
      currentTranslateY.current = value;
    });
    return () => {
      translateY.removeListener(id);
    };
  }, [translateY]);

  const animateToOpen = () => {
    Animated.parallel([
      Animated.timing(backdropOpacity, {
        toValue: 1,
        duration: 260,
        easing: Easing.out(Easing.quad),
        useNativeDriver: true,
      }),
      Animated.timing(translateY, {
        toValue: 0,
        duration: 320,
        easing: Easing.out(Easing.cubic),
        useNativeDriver: true,
      }),
    ]).start();
  };

  const animateToClosed = () => {
    Animated.parallel([
      Animated.timing(backdropOpacity, {
        toValue: 0,
        duration: 180,
        easing: Easing.in(Easing.quad),
        useNativeDriver: true,
      }),
      Animated.timing(translateY, {
        toValue: sheetHeightRef.current + 40,
        duration: 240,
        easing: Easing.in(Easing.cubic),
        useNativeDriver: true,
      }),
    ]).start(() => {
      setMounted(false);
    });
  };

  useEffect(() => {
    if (visible) {
      setMounted(true);
      requestAnimationFrame(() => {
        translateY.setValue(sheetHeightRef.current + 40);
        backdropOpacity.setValue(0);
        animateToOpen();
      });
      return;
    }

    if (mounted) {
      animateToClosed();
    }
  }, [backdropOpacity, mounted, translateY, visible]);

  const panResponder = useMemo(
    () =>
      PanResponder.create({
        onMoveShouldSetPanResponder: (_, gestureState) =>
          gestureState.dy > 6 && Math.abs(gestureState.dy) > Math.abs(gestureState.dx),
        onPanResponderMove: (_, gestureState) => {
          const nextY = Math.max(0, gestureState.dy);
          translateY.setValue(nextY);
          const progress = 1 - Math.min(nextY / Math.max(sheetHeightRef.current, 1), 1);
          backdropOpacity.setValue(progress);
        },
        onPanResponderRelease: (_, gestureState) => {
          const shouldClose =
            gestureState.dy > sheetHeightRef.current * 0.22 || gestureState.vy > 1.15;

          if (shouldClose) {
            onClose();
            return;
          }

          Animated.parallel([
            Animated.timing(backdropOpacity, {
              toValue: 1,
              duration: 160,
              useNativeDriver: true,
            }),
            Animated.spring(translateY, {
              toValue: 0,
              damping: 22,
              stiffness: 240,
              mass: 0.9,
              useNativeDriver: true,
            }),
          ]).start();
        },
        onPanResponderTerminate: () => {
          Animated.parallel([
            Animated.timing(backdropOpacity, {
              toValue: 1,
              duration: 160,
              useNativeDriver: true,
            }),
            Animated.spring(translateY, {
              toValue: 0,
              damping: 22,
              stiffness: 240,
              mass: 0.9,
              useNativeDriver: true,
            }),
          ]).start();
        },
      }),
    [backdropOpacity, onClose, translateY],
  );

  if (!mounted) {
    return null;
  }

  return (
    <Modal
      transparent
      visible={mounted}
      animationType="none"
      statusBarTranslucent
      onRequestClose={onClose}>
      <View className="flex-1 justify-end">
        <Pressable className="absolute inset-0" onPress={onClose}>
          <Animated.View
            className="flex-1"
            style={{backgroundColor: 'transparent', opacity: backdropOpacity}}
          />
        </Pressable>

        <Animated.View
          className="rounded-t-[28px] px-5 pb-4 pt-3"
          style={{
            backgroundColor: colors.surface,
            maxHeight: Math.max(windowHeight - insets.top - 16, HIDDEN_OFFSET),
            paddingBottom: Math.max(insets.bottom, 16),
            transform: [{translateY}],
          }}
          onLayout={(event) => {
            sheetHeightRef.current = event.nativeEvent.layout.height || HIDDEN_OFFSET;
          }}>
          <View
            className="mb-5 items-center"
            {...panResponder.panHandlers}>
            <View
              className="h-1.5 w-12 rounded-full"
              style={{backgroundColor: colors.border}}
            />
            {title || headerRight ? (
              <View className="mt-4 min-h-7 w-full justify-center">
                {title ? (
                  <Text
                    className="px-16 text-center"
                    style={{
                      fontSize: fontSizes.lg,
                      fontWeight: '600',
                      color: colors.textPrimary,
                    }}>
                    {title}
                  </Text>
                ) : null}
                {headerRight ? (
                  <View className="absolute right-0 top-0 bottom-0 justify-center">
                    {headerRight}
                  </View>
                ) : null}
              </View>
            ) : null}
          </View>

          {children}
        </Animated.View>
      </View>
    </Modal>
  );
}
