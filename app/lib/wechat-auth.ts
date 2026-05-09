import {NativeModules, Platform} from 'react-native';

type WeChatNativeAuthModule = {
  signIn: (appId: string) => Promise<string>;
  clearAuthState: () => Promise<void>;
};

const WECHAT_AUTH_MODULE_NAME = 'QuickQueWeChatAuth';

function getWeChatAuthModule(): WeChatNativeAuthModule {
  const module = NativeModules[WECHAT_AUTH_MODULE_NAME] as WeChatNativeAuthModule | undefined;
  if (!module) {
    throw new Error('WeChat Sign-In native module is not available');
  }
  return module;
}

export async function signInWithWeChat(appId: string): Promise<string> {
  if (Platform.OS !== 'android') {
    throw new Error('微信登录当前仅支持 Android');
  }
  if (!appId.trim()) {
    throw new Error('微信登录尚未配置');
  }
  return getWeChatAuthModule().signIn(appId.trim());
}

export async function clearWeChatAuthState() {
  if (Platform.OS !== 'android') {
    return;
  }
  try {
    await getWeChatAuthModule().clearAuthState();
  } catch {
    // Ignore cleanup failures so local logout still works.
  }
}
