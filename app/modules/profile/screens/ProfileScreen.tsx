import {Ionicons} from '@expo/vector-icons';
import {useRouter} from 'expo-router';
import type {ComponentProps, RefObject} from 'react';
import {useRef, useState} from 'react';
import {Modal, Pressable, Text, View, useWindowDimensions} from 'react-native';

import {ConfirmModal} from '@/components/ConfirmModal';
import {Screen} from '@/components/Screen';
import {useI18n} from '@/i18n/useI18n';
import {languageLabelMap} from '@/i18n/languages';
import {API_BASE_URL} from '@/lib/api';
import {useAuthStore} from '@/store/auth';
import {usePreferencesStore} from '@/store/preferences';
import type {AppColors} from '@/theme/colors';
import type {AppearanceMode} from '@/theme/colors';
import {fontSizes} from '@/theme/typography';
import {useAppTheme} from '@/theme/useAppTheme';

type IconName = ComponentProps<typeof Ionicons>['name'];

type SettingItem = {
  icon: IconName;
  label: string;
  value?: string;
  external?: boolean;
  onPress?: () => void;
};

type AppearanceOption = {
  key: AppearanceMode;
  label: string;
  icon: IconName;
};

type MenuPosition = {
  top: number;
  left: number;
  width: number;
};

function SettingsGroup({items, colors}: {items: SettingItem[]; colors: AppColors}) {
  return (
    <View
      className="overflow-hidden rounded-[12px]"
      style={{backgroundColor: colors.surface}}>
      {items.map((item, index) => (
        <View key={item.label}>
          {index > 0 ? (
            <View className="ml-14 h-px" style={{backgroundColor: colors.divider}} />
          ) : null}
          <SettingRow item={item} colors={colors} onPress={item.onPress} />
        </View>
      ))}
    </View>
  );
}

function SettingRow({
  item,
  colors,
  onPress,
  chevronName,
  rowRef,
  valueColor,
}: {
  item: SettingItem;
  colors: AppColors;
  onPress?: () => void;
  chevronName?: IconName;
  rowRef?: RefObject<View | null>;
  valueColor?: string;
}) {
  return (
    <View ref={rowRef}>
      <Pressable
        className="flex-row items-center gap-4 px-5 py-3"
        onPress={onPress}>
        <View className="h-7 w-7 items-center justify-center">
          <Ionicons name={item.icon} size={22} color={colors.icon} />
        </View>
        <Text
          className="flex-1 font-normal"
          style={{fontSize: fontSizes.md, color: colors.textPrimary}}>
          {item.label}
        </Text>
        {item.value ? (
          <Text style={{fontSize: fontSizes.sm, color: valueColor ?? colors.textMuted}}>
            {item.value}
          </Text>
        ) : null}
        <Ionicons
          name={chevronName ?? (item.external ? 'open-outline' : 'chevron-forward')}
          size={18}
          color={colors.iconSubtle}
        />
      </Pressable>
    </View>
  );
}

function PreferenceSettingsGroup({
  colors,
  languageLabel,
  appearanceLabel,
  cacheSize,
  showAppearanceMenu,
  onLanguagePress,
  onAppearancePress,
  appearanceAnchorRef,
  labels,
}: {
  colors: AppColors;
  languageLabel: string;
  appearanceLabel: string;
  cacheSize: string;
  showAppearanceMenu: boolean;
  onLanguagePress: () => void;
  onAppearancePress: () => void;
  appearanceAnchorRef: RefObject<View | null>;
  labels: {
    language: string;
    appearance: string;
    clearCache: string;
  };
}) {
  return (
    <View
      className="overflow-hidden rounded-[12px]"
      style={{backgroundColor: colors.surface}}>
      <SettingRow
        item={{icon: 'language-outline', label: labels.language, value: languageLabel}}
        colors={colors}
        onPress={onLanguagePress}
      />
      <View className="ml-14 h-px" style={{backgroundColor: colors.divider}} />
      <SettingRow
        colors={colors}
        item={{
          icon: 'contrast-outline',
          label: labels.appearance,
          value: appearanceLabel,
        }}
        chevronName={showAppearanceMenu ? 'chevron-up' : 'chevron-forward'}
        onPress={onAppearancePress}
        rowRef={appearanceAnchorRef}
        valueColor={showAppearanceMenu ? colors.brand : colors.textMuted}
      />
      <View className="ml-14 h-px" style={{backgroundColor: colors.divider}} />
      <SettingRow
        item={{icon: 'trash-outline', label: labels.clearCache, value: cacheSize}}
        colors={colors}
      />
    </View>
  );
}

function AppearanceMenu({
  colors,
  visible,
  position,
  selectedAppearance,
  options,
  onClose,
  onSelect,
}: {
  colors: AppColors;
  visible: boolean;
  position: MenuPosition | null;
  selectedAppearance: AppearanceMode;
  options: AppearanceOption[];
  onClose: () => void;
  onSelect: (appearance: AppearanceMode) => void;
}) {
  if (!visible || !position) {
    return null;
  }

  return (
    <Modal
      transparent
      visible={visible}
      animationType="fade"
      statusBarTranslucent
      onRequestClose={onClose}>
      <View className="flex-1">
        <Pressable
          className="absolute inset-0"
          style={{backgroundColor: colors.overlaySoft}}
          onPress={onClose}
        />

        <View
          className="absolute overflow-hidden rounded-[12px]"
          style={{
            top: position.top,
            left: position.left,
            width: position.width,
            backgroundColor: colors.surface,
            shadowColor: colors.shadow,
            shadowOffset: {width: 0, height: 10},
            shadowOpacity: 0.1,
            shadowRadius: 18,
            elevation: 12,
          }}>
          {options.map((option, index) => {
            const isSelected = selectedAppearance === option.key;

            return (
              <View key={option.key}>
                {index > 0 ? (
                  <View className="ml-5 h-px" style={{backgroundColor: colors.divider}} />
                ) : null}
                <Pressable
                  className="flex-row items-center px-5 py-4"
                  onPress={() => onSelect(option.key)}>
                  <View className="mr-1 h-6 w-6 items-center justify-center">
                    {isSelected ? (
                      <Ionicons name="checkmark" size={20} color={colors.brand} />
                    ) : null}
                  </View>
                  <Text
                    className="flex-1"
                    style={{
                      fontSize: fontSizes.md,
                      fontWeight: '500',
                      color: isSelected ? colors.brand : colors.textPrimary,
                    }}>
                    {option.label}
                  </Text>
                  <Ionicons
                    name={option.icon}
                    size={20}
                    color={isSelected ? colors.brand : colors.icon}
                  />
                </Pressable>
              </View>
            );
          })}
        </View>
      </View>
    </Modal>
  );
}

function PlanCard({
  colors,
  planLabel,
  upgradeLabel,
  creditsLabel,
  onUpgrade,
  onOpenUsage,
}: {
  colors: AppColors;
  planLabel: string;
  upgradeLabel: string;
  creditsLabel: string;
  onUpgrade: () => void;
  onOpenUsage: () => void;
}) {
  return (
    <View
      className="overflow-hidden rounded-[12px]"
      style={{backgroundColor: colors.surface}}>
      <View className="flex-row items-center justify-between px-5 pb-3 pt-3">
        <Text
          style={{
            fontSize: fontSizes.xl,
            fontWeight: '700',
            color: colors.textPrimary,
          }}>
          {planLabel}
        </Text>
        <Pressable
          className="rounded-xl px-4 py-2"
          style={{backgroundColor: colors.brand}}
          onPress={onUpgrade}>
          <Text style={{fontSize: fontSizes.sm, fontWeight: '600', color: colors.surface}}>
            {upgradeLabel}
          </Text>
        </Pressable>
      </View>
      <View className="mx-5 h-px" style={{backgroundColor: colors.divider}} />
      <Pressable
        className="flex-row items-center gap-4 px-5 py-3"
        onPress={onOpenUsage}>
        <View className="h-7 w-7 items-center justify-center">
          <Ionicons name="sparkles-outline" size={21} color={colors.icon} />
        </View>
        <Text
          className="flex-1"
          style={{fontSize: fontSizes.md, color: colors.textPrimary}}>
          {creditsLabel}
        </Text>
        <Text style={{fontSize: fontSizes.md, color: colors.textMuted}}>2860</Text>
        <Ionicons name="chevron-forward" size={18} color={colors.iconSubtle} />
      </Pressable>
    </View>
  );
}

function LogoutCard({
  colors,
  label,
  onPress,
}: {
  colors: AppColors;
  label: string;
  onPress: () => void;
}) {
  return (
    <Pressable
      className="flex-row items-center gap-4 rounded-[28px] px-5 py-4"
      style={{backgroundColor: colors.surface}}
      onPress={onPress}>
      <View className="h-7 w-7 items-center justify-center">
        <Ionicons name="log-out-outline" size={22} color={colors.textPrimary} />
      </View>
      <Text
        style={{fontSize: fontSizes.md, color: colors.textPrimary}}>
        {label}
      </Text>
    </Pressable>
  );
}

export default function ProfileScreen() {
  const router = useRouter();
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const appearance = usePreferencesStore((state) => state.appearance);
  const language = usePreferencesStore((state) => state.language);
  const setAppearance = usePreferencesStore((state) => state.setAppearance);
  const {colors} = useAppTheme();
  const {t} = useI18n();
  const [showLogoutModal, setShowLogoutModal] = useState(false);
  const [showAppearanceMenu, setShowAppearanceMenu] = useState(false);
  const [appearanceMenuPosition, setAppearanceMenuPosition] = useState<MenuPosition | null>(
    null,
  );
  const appearanceAnchorRef = useRef<View>(null);
  const {width: windowWidth} = useWindowDimensions();
  const workspaceSettings: SettingItem[] = [
    // {icon: 'share-social-outline', label: t('profile.shareWithFriends')},
    // {icon: 'calendar-outline', label: t('profile.scheduledTasks')},
    // {icon: 'book-outline', label: t('profile.knowledge')},
    // {icon: 'mail-outline', label: t('profile.mailManus')},
    // {icon: 'server-outline', label: t('profile.dataManagement')},
    // {icon: 'browsers-outline', label: t('profile.cloudBrowser')},
    {icon: 'construct-outline', label: t('profile.skills'), onPress: () => router.push('/skills')},
    {
      icon: 'hardware-chip-outline',
      label: t('profile.modelConfiguration'),
      onPress: () => router.push('/models'),
    },
    ...(user?.fluxaSite === 'paid' || user?.fluxaSite === 'free'
      ? [
          {
            icon: 'layers-outline' as IconName,
            label: t('profile.fluxaModelGroups'),
            onPress: () => router.push('/fluxa-model-groups'),
          },
        ]
      : []),
    // {icon: 'extension-puzzle-outline', label: t('profile.integrations')},
  ];
  const supportSettings: SettingItem[] = [
    // {icon: 'document-text-outline', label: t('profile.playbook'), external: true},
    // {icon: 'heart-outline', label: t('profile.rateApp'), external: true},
    // {icon: 'help-circle-outline', label: t('profile.getHelp'), external: true},
    {icon: 'information-circle-outline', label: t('profile.version'), value: 'v0.1.0'},
  ];
  const appearanceOptions: AppearanceOption[] = [
    {key: 'system', label: t('appearance.system'), icon: 'contrast-outline'},
    {key: 'light', label: t('appearance.light'), icon: 'sunny-outline'},
    {key: 'dark', label: t('appearance.dark'), icon: 'moon-outline'},
  ];
  const selectedAppearanceLabel =
    appearanceOptions.find((option) => option.key === appearance)?.label ??
    t('appearance.system');

  const toggleAppearanceMenu = () => {
    if (showAppearanceMenu) {
      setShowAppearanceMenu(false);
      return;
    }

    appearanceAnchorRef.current?.measureInWindow((x, y, width, height) => {
      const menuWidth = Math.min(180, windowWidth - 32);
      const menuLeft = Math.max(
        16,
        Math.min(x + width - menuWidth, windowWidth - menuWidth - 16),
      );

      setAppearanceMenuPosition({
        top: y + height + 8,
        left: menuLeft,
        width: menuWidth,
      });
      setShowAppearanceMenu(true);
    });
  };

  return (
    <>
      <Screen scroll>
        <View className="gap-6">
          <View className="gap-2">
            <Text
              className="font-bold"
              style={{fontSize: 30, color: colors.textPrimary}}>
              {t('profile.title')}
            </Text>
            <Text
              className="leading-7"
              style={{fontSize: fontSizes.md, color: colors.textSecondary}}>
              {t('profile.description')}
            </Text>
          </View>

          <View
            className="gap-3 rounded-[12px] p-5"
            style={{backgroundColor: colors.surface}}>
            <Text
              className="uppercase"
              style={{fontSize: fontSizes.xs, color: colors.textTertiary}}>
              {t('profile.user')}
            </Text>
            <Text
              style={{fontSize: fontSizes.lg, fontWeight: '600', color: colors.textPrimary}}>
              {user?.displayName}
            </Text>
            <Text style={{fontSize: fontSizes.sm, color: colors.textSecondary}}>
              {user?.username}
            </Text>
          </View>

          {/* <View
            className="gap-3 rounded-[12px] p-5"
            style={{backgroundColor: colors.surface}}>
            <Text
              className="uppercase"
              style={{fontSize: fontSizes.xs, color: colors.textTertiary}}>
              {t('profile.api')}
            </Text>
            <Text
              style={{
                fontSize: fontSizes.sm,
                lineHeight: 24,
                color: colors.textSecondary,
              }}>
              {API_BASE_URL}
            </Text>
          </View> */}

          <PlanCard
            colors={colors}
            planLabel={t('profile.free')}
            upgradeLabel={t('profile.upgrade')}
            creditsLabel={t('profile.credits')}
            onUpgrade={() => router.push('/upgrade')}
            onOpenUsage={() => router.push('/usage')}
          />

          <SettingsGroup
            colors={colors}
            items={[{icon: 'person-circle-outline', label: t('profile.account')}]}
          />

          <SettingsGroup colors={colors} items={workspaceSettings} />

          <PreferenceSettingsGroup
            colors={colors}
            languageLabel={languageLabelMap[language]}
            appearanceLabel={selectedAppearanceLabel}
            cacheSize="2 MB"
            showAppearanceMenu={showAppearanceMenu}
            onLanguagePress={() => router.push('/language')}
            onAppearancePress={toggleAppearanceMenu}
            appearanceAnchorRef={appearanceAnchorRef}
            labels={{
              language: t('profile.language'),
              appearance: t('profile.appearance'),
              clearCache: t('profile.clearCache'),
            }}
          />

          <SettingsGroup colors={colors} items={supportSettings} />

          <LogoutCard
            colors={colors}
            label={t('profile.logOut')}
            onPress={() => setShowLogoutModal(true)}
          />
        </View>
      </Screen>

      <ConfirmModal
        visible={showLogoutModal}
        title={t('profile.logoutConfirmTitle')}
        body={t('profile.logoutConfirmBody')}
        cancelLabel={t('common.cancel')}
        confirmLabel={t('profile.logOut')}
        onCancel={() => setShowLogoutModal(false)}
        onConfirm={async () => {
          setShowLogoutModal(false);
          await logout();
          router.replace('/(auth)/login');
        }}
      />

      <AppearanceMenu
        colors={colors}
        visible={showAppearanceMenu}
        position={appearanceMenuPosition}
        selectedAppearance={appearance}
        options={appearanceOptions}
        onClose={() => setShowAppearanceMenu(false)}
        onSelect={(nextAppearance) => {
          void setAppearance(nextAppearance);
          setShowAppearanceMenu(false);
        }}
      />
    </>
  );
}
