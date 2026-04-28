import {useCallback, useEffect, useRef, useState} from 'react';
import {Animated, Text, View} from 'react-native';
import {useSafeAreaInsets} from 'react-native-safe-area-context';

import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

type TatosProps = {
  visible: boolean;
  title?: string;
  body: string;
  opacity: Animated.Value;
  translateY: Animated.Value;
};

type TatosOptions = {
  title?: string;
  body: string;
  duration?: number;
};

type TatosState = {
  visible: boolean;
  title?: string;
  body: string;
};

const DEFAULT_DURATION_MS = 2600;

function buildInitialState(): TatosState {
  return {
    visible: false,
    title: undefined,
    body: '',
  };
}

export function Tatos({visible, title, body, opacity, translateY}: TatosProps) {
  const {colors} = useAppTheme();
  const insets = useSafeAreaInsets();

  if (!visible) {
    return null;
  }

  return (
    <View pointerEvents="none" className="absolute inset-0">
      <Animated.View
        className="absolute left-5 right-5 rounded-[18px] px-4 py-3"
        style={{
          bottom: Math.max(insets.bottom + 20, 28),
          backgroundColor: colors.surface,
          borderWidth: 1,
          borderColor: colors.divider,
          shadowColor: colors.shadow,
          shadowOffset: {width: 0, height: 10},
          shadowOpacity: 0.12,
          shadowRadius: 18,
          elevation: 10,
          opacity,
          transform: [{translateY}],
        }}>
        {title ? (
          <Text style={{fontSize: fontSizes.sm, fontWeight: '600', color: colors.textPrimary}}>
            {title}
          </Text>
        ) : null}
        <Text
          className={title ? 'mt-1.5' : undefined}
          style={{
            fontSize: fontSizes.sm,
            lineHeight: 22,
            color: title ? colors.textSecondary : colors.textPrimary,
          }}>
          {body}
        </Text>
      </Animated.View>
    </View>
  );
}

export function useTatos() {
  const [state, setState] = useState<TatosState>(() => buildInitialState());
  const opacity = useRef(new Animated.Value(0)).current;
  const translateY = useRef(new Animated.Value(16)).current;
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearTimer = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const hideTatos = useCallback(() => {
    clearTimer();
    Animated.parallel([
      Animated.timing(opacity, {
        toValue: 0,
        duration: 180,
        useNativeDriver: true,
      }),
      Animated.timing(translateY, {
        toValue: 16,
        duration: 180,
        useNativeDriver: true,
      }),
    ]).start(() => {
      setState(buildInitialState());
    });
  }, [clearTimer, opacity, translateY]);

  const showTatos = useCallback(
    ({title, body, duration = DEFAULT_DURATION_MS}: TatosOptions) => {
      clearTimer();
      opacity.stopAnimation();
      translateY.stopAnimation();
      opacity.setValue(0);
      translateY.setValue(16);
      setState({
        visible: true,
        title,
        body,
      });
      Animated.parallel([
        Animated.timing(opacity, {
          toValue: 1,
          duration: 180,
          useNativeDriver: true,
        }),
        Animated.timing(translateY, {
          toValue: 0,
          duration: 180,
          useNativeDriver: true,
        }),
      ]).start();
      timerRef.current = setTimeout(() => {
        hideTatos();
      }, duration);
    },
    [clearTimer, hideTatos, opacity, translateY],
  );

  useEffect(() => () => clearTimer(), [clearTimer]);

  return {
    showTatos,
    hideTatos,
    tatosNode: (
      <Tatos
        visible={state.visible}
        title={state.title}
        body={state.body}
        opacity={opacity}
        translateY={translateY}
      />
    ),
  };
}
