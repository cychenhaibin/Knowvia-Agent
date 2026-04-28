import {Redirect} from 'expo-router';

import {useAuthStore} from '@/store/auth';

export default function IndexRoute() {
  const user = useAuthStore((state) => state.user);

  return <Redirect href={user ? '/(tabs)/runs' : '/(auth)/login'} />;
}
