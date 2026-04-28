import {NativeModules, Platform} from 'react-native';

type GoogleNativeAuthResult = {
  idToken: string;
  email?: string;
  displayName?: string;
  avatarUrl?: string;
};

type GoogleNativeAuthModule = {
  signIn: (serverClientId: string) => Promise<GoogleNativeAuthResult>;
  clearCredentialState: () => Promise<void>;
};

const GOOGLE_AUTH_MODULE_NAME = 'QuickQueGoogleAuth';

function getGoogleAuthModule(): GoogleNativeAuthModule {
  const module = NativeModules[GOOGLE_AUTH_MODULE_NAME] as GoogleNativeAuthModule | undefined;
  if (!module) {
    throw new Error('Google Sign-In native module is not available');
  }
  return module;
}

export async function signInWithGoogle(serverClientId: string) {
  if (Platform.OS !== 'android') {
    throw new Error('Google Sign-In is currently implemented for Android only');
  }
  if (!serverClientId.trim()) {
    throw new Error('Google Sign-In is not configured');
  }
  return getGoogleAuthModule().signIn(serverClientId.trim());
}

export async function clearGoogleCredentialState() {
  if (Platform.OS !== 'android') {
    return;
  }
  try {
    await getGoogleAuthModule().clearCredentialState();
  } catch {
    // Ignore cleanup failures so local logout still works.
  }
}
