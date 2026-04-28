import {Text, View} from 'react-native';

import {useI18n} from '@/i18n/useI18n';
import type {RunStatus, StepStatus} from '@/types/api';
import {useAppTheme} from '@/theme/useAppTheme';

export function StatusBadge({
  status,
  variant = 'default',
}: {
  status: RunStatus | StepStatus;
  variant?: 'default' | 'neutral';
}) {
  const {themeName, colors} = useAppTheme();
  const {statusLabel} = useI18n();

  if (variant === 'neutral') {
    const active = status === 'running' || status === 'planning';
    const done = status === 'completed';
    const failed = status === 'failed';
    return (
      <View
        className="self-start rounded-full px-3 py-1"
        style={{
          backgroundColor: active ? colors.brandSoft : colors.surfaceMuted,
          borderWidth: 1,
          borderColor: active ? colors.brand : done ? colors.border : failed ? colors.border : colors.border,
        }}>
        <Text
          className="text-xs font-semibold uppercase"
          style={{
            color: active ? colors.brand : colors.textPrimary,
          }}>
          {statusLabel(status)}
        </Text>
      </View>
    );
  }

  const tone =
    status === 'completed'
      ? themeName === 'dark'
        ? {backgroundColor: 'rgba(52, 211, 153, 0.18)', color: '#6ee7b7'}
        : {backgroundColor: '#dcfce7', color: '#15803d'}
      : status === 'failed'
        ? themeName === 'dark'
          ? {backgroundColor: 'rgba(248, 113, 113, 0.16)', color: '#fca5a5'}
          : {backgroundColor: '#fee2e2', color: '#b91c1c'}
        : status === 'running' || status === 'planning'
          ? themeName === 'dark'
            ? {backgroundColor: 'rgba(251, 146, 60, 0.18)', color: '#fdba74'}
            : {backgroundColor: '#ffedd5', color: '#c2410c'}
          : themeName === 'dark'
            ? {backgroundColor: 'rgba(148, 163, 184, 0.16)', color: '#cbd5e1'}
            : {backgroundColor: '#e5e7eb', color: '#374151'};

  return (
    <View
      className="self-start rounded-full px-3 py-1"
      style={{backgroundColor: tone.backgroundColor}}>
      <Text className="text-xs font-semibold uppercase" style={{color: tone.color}}>
        {statusLabel(status)}
      </Text>
    </View>
  );
}
