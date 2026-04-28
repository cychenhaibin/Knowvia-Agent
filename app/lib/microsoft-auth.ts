import {NativeModules, Platform} from 'react-native';

type MicrosoftNativeAuthResult = {
  idToken: string;
  email?: string;
  displayName?: string;
};

type MicrosoftNativeAuthModule = {
  signIn: () => Promise<MicrosoftNativeAuthResult>;
  clearAccountState: () => Promise<void>;
};

const MICROSOFT_AUTH_MODULE_NAME = 'QuickQueMicrosoftAuth';

function getMicrosoftAuthModule(): MicrosoftNativeAuthModule {
  const module = NativeModules[MICROSOFT_AUTH_MODULE_NAME] as MicrosoftNativeAuthModule | undefined;
  if (!module) {
    throw new Error('Microsoft Sign-In native module is not available');
  }
  return module;
}

export async function signInWithMicrosoft() {
  if (Platform.OS !== 'android') {
    throw new Error('Microsoft Sign-In is currently implemented for Android only');
  }
  return getMicrosoftAuthModule().signIn();
}

export async function clearMicrosoftAccountState() {
  if (Platform.OS !== 'android') {
    return;
  }
  try {
    await getMicrosoftAuthModule().clearAccountState();
  } catch {
    // Ignore cleanup failures so local logout still works.
  }
}
