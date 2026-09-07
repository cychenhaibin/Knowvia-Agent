import {useMutation, useQuery, useQueryClient} from '@tanstack/react-query';
import {useRouter} from 'expo-router';
import {Pressable, Text, View} from 'react-native';
import {useState} from 'react';

import {ModePicker} from '@/components/ModePicker';
import {PrimaryButton} from '@/components/PrimaryButton';
import {Screen} from '@/components/Screen';
import {TextField} from '@/components/TextField';
import {useI18n} from '@/i18n/useI18n';
import {api} from '@/lib/api';
import {useAuthStore} from '@/store/auth';
import {useComposerStore} from '@/store/composer';
import {useAppTheme} from '@/theme/useAppTheme';

export default function ComposeScreen() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const accessToken = useAuthStore((state) => state.accessToken)!;
  const {title, goal, mode, setTitle, setGoal, setMode, reset} = useComposerStore();
  const [selectedConnectionIds, setSelectedConnectionIds] = useState<string[]>([]);
  const {colors} = useAppTheme();
  const {t} = useI18n();

  const connectionsQuery = useQuery({
    queryKey: ['knowledge-connections'],
    queryFn: () => api.listConnections(accessToken),
  });

  const mutation = useMutation({
    mutationFn: () =>
      api.createRun(accessToken, {
        title,
        goal,
        mode,
        knowledge_connection_ids: selectedConnectionIds,
      }),
    onSuccess: async (run) => {
      reset();
      await queryClient.invalidateQueries({queryKey: ['runs']});
      if (run.taskSessionId && run.taskPrompt) {
        router.replace({
          pathname: '/(tabs)/runs',
          params: {
            runId: run.id,
            taskSessionId: run.taskSessionId,
            taskPrompt: run.taskPrompt,
          },
        });
        return;
      }
      router.replace(`/runs/${run.id}`);
    },
  });

  const toggleConnection = (connectionId: string) => {
    setSelectedConnectionIds((current) =>
      current.includes(connectionId)
        ? current.filter((id) => id !== connectionId)
        : [...current, connectionId],
    );
  };

  return (
    <Screen scroll>
      <View className="gap-6 pb-12">
        <View className="gap-2">
          <Text className="text-3xl font-bold" style={{color: colors.textPrimary}}>
            {t('compose.title')}
          </Text>
          <Text className="text-base leading-7" style={{color: colors.textSecondary}}>
            {t('compose.description')}
          </Text>
        </View>

        <View className="gap-5 rounded-[12px] p-5" style={{backgroundColor: colors.surface}}>
          <TextField
            label={t('compose.runTitle')}
            value={title}
            onChangeText={setTitle}
            placeholder={t('compose.placeholder.title')}
          />
          <TextField
            label={t('compose.goal')}
            value={goal}
            onChangeText={setGoal}
            placeholder={t('compose.placeholder.goal')}
            multiline
          />
          <ModePicker value={mode} onChange={setMode} />

          <View className="gap-3">
            <Text className="text-md font-semibold" style={{color: colors.textPrimary}}>
              {t('compose.knowledgeConnections')}
            </Text>
            <View className="flex-row flex-wrap gap-2">
              {connectionsQuery.data?.items.map((connection) => {
                const selected = selectedConnectionIds.includes(connection.id);
                return (
                  <Pressable
                    key={connection.id}
                    className="rounded-full px-3 py-2"
                    style={{
                      backgroundColor: selected ? colors.brand : colors.brandSoft,
                    }}
                    onPress={() => toggleConnection(connection.id)}>
                    <Text
                      className="font-normal"
                      style={{color: selected ? colors.surface : colors.brand}}>
                      {connection.name}
                    </Text>
                  </Pressable>
                );
              })}
            </View>
            {connectionsQuery.error ? (
              <Text className="text-sm text-red-600">
                {connectionsQuery.error instanceof Error
                  ? connectionsQuery.error.message
                  : t('compose.loadConnectionsFailed')}
              </Text>
            ) : null}
          </View>

          <PrimaryButton
            label={mutation.isPending ? t('compose.creatingRun') : t('compose.startRun')}
            disabled={!goal.trim() || mutation.isPending}
            onPress={() => mutation.mutate()}
          />
          {mutation.error ? (
            <Text className="text-sm text-red-600">
              {mutation.error instanceof Error ? mutation.error.message : t('compose.createRunFailed')}
            </Text>
          ) : null}
        </View>
      </View>
    </Screen>
  );
}
