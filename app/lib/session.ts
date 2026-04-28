import * as SecureStore from 'expo-secure-store';

const ACCESS_TOKEN_KEY = 'quickque-agent-access-token';
const REFRESH_TOKEN_KEY = 'quickque-agent-refresh-token';
const USER_KEY = 'quickque-agent-user';

export async function saveSession(session: {
  accessToken: string;
  refreshToken: string;
  user: string;
}) {
  await SecureStore.setItemAsync(ACCESS_TOKEN_KEY, session.accessToken);
  await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, session.refreshToken);
  await SecureStore.setItemAsync(USER_KEY, session.user);
}

export async function loadSession() {
  const [accessToken, refreshToken, user] = await Promise.all([
    SecureStore.getItemAsync(ACCESS_TOKEN_KEY),
    SecureStore.getItemAsync(REFRESH_TOKEN_KEY),
    SecureStore.getItemAsync(USER_KEY),
  ]);

  if (!accessToken || !refreshToken || !user) {
    return null;
  }

  return {accessToken, refreshToken, user};
}

export async function clearSession() {
  await Promise.all([
    SecureStore.deleteItemAsync(ACCESS_TOKEN_KEY),
    SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY),
    SecureStore.deleteItemAsync(USER_KEY),
  ]);
}
