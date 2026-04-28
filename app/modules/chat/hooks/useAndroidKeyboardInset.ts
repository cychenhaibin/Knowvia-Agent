import {useEffect, useState} from 'react';
import {Keyboard, Platform} from 'react-native';

export function useAndroidKeyboardInset(bottomInset: number) {
  const [keyboardInset, setKeyboardInset] = useState(0);

  useEffect(() => {
    if (Platform.OS !== 'android') {
      return;
    }

    const handleKeyboardShow = (event: {endCoordinates?: {height?: number}}) => {
      const keyboardHeight = event.endCoordinates?.height ?? 0;
      setKeyboardInset(Math.max(keyboardHeight - bottomInset, 0));
    };

    const handleKeyboardHide = () => {
      setKeyboardInset(0);
    };

    const showSubscription = Keyboard.addListener('keyboardDidShow', handleKeyboardShow);
    const hideSubscription = Keyboard.addListener('keyboardDidHide', handleKeyboardHide);

    return () => {
      showSubscription.remove();
      hideSubscription.remove();
    };
  }, [bottomInset]);

  return keyboardInset;
}
