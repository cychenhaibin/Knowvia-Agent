import Constants from 'expo-constants';
import {Platform} from 'react-native';
import EventSource from 'react-native-sse';

import {useAuthStore} from '@/store/auth';
import type {
  ChatSkill,
  ChatSession,
  ChatModelPurpose,
  ChatStreamEvent,
  FluxALoginResult,
  FluxASite,
  KnowledgeConnection,
  KnowledgeConnectionDetails,
  KnowledgeSyncJob,
  PersistedChatMessage,
  Run,
  RunDetails,
  RunEvent,
  SessionPayload,
  Skill,
  SkillImportResult,
  UserChatModel,
  UserChatModelGroups,
  User,
} from '@/types/api';

const DEFAULT_API_PORT = '8088';
const DEFAULT_API_PATH = '/v1';
const DEV_CLIENT_HOSTNAME = 'expo-development-client';

function normalizeBaseUrl(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '');
}

function normalizeHost(host: string | null | undefined): string | null {
  if (!host) {
    return null;
  }

  const normalized = host.trim().replace(/^\[|\]$/g, '');
  if (!normalized || normalized === DEV_CLIENT_HOSTNAME) {
    return null;
  }

  return normalized;
}

function addDevHost(hosts: Set<string>, host: string | null | undefined) {
  const normalized = normalizeHost(host);
  if (normalized) {
    hosts.add(normalized);
  }
}

function parseUri(candidate: string): URL | null {
  try {
    return new URL(candidate);
  } catch {
    try {
      return new URL(`http://${candidate}`);
    } catch {
      return null;
    }
  }
}

function collectHostsFromUri(rawValue: string | null | undefined, hosts: Set<string>) {
  if (!rawValue) {
    return;
  }

  const value = rawValue.trim();
  if (!value) {
    return;
  }

  const parsed = parseUri(value);
  if (!parsed) {
    return;
  }

  addDevHost(hosts, parsed.hostname);

  for (const key of ['url', 'bundleUrl']) {
    const nestedValue = parsed.searchParams.get(key);
    if (nestedValue) {
      collectHostsFromUri(decodeURIComponent(nestedValue), hosts);
    }
  }
}

function getExpoDevHosts(): string[] {
  const hosts = new Set<string>();

  collectHostsFromUri(Constants.expoConfig?.hostUri, hosts);
  collectHostsFromUri(Constants.linkingUri, hosts);
  collectHostsFromUri(Constants.experienceUrl, hosts);
  collectHostsFromUri(Constants.intentUri, hosts);

  return Array.from(hosts);
}

function getExpoDevHost(): string | null {
  return getExpoDevHosts()[0] ?? null;
}

function getLocalDevHost(): string {
  return '127.0.0.1';
}

function resolveConfiguredUrl(configuredUrl: string): string {
  try {
    const url = new URL(configuredUrl);
    const isLoopbackHost = url.hostname === '127.0.0.1' || url.hostname === 'localhost';

    if (__DEV__ && isLoopbackHost) {
      url.hostname = getExpoDevHost() ?? getLocalDevHost();
    }

    if (!url.pathname || url.pathname === '/') {
      url.pathname = DEFAULT_API_PATH;
    }

    return normalizeBaseUrl(url.toString());
  } catch {
    return normalizeBaseUrl(configuredUrl);
  }
}

function resolveApiBaseUrl(): string {
  const configuredUrl = process.env.EXPO_PUBLIC_API_BASE_URL?.trim();
  if (configuredUrl) {
    return resolveConfiguredUrl(configuredUrl);
  }

  const host = getExpoDevHost() ?? getLocalDevHost();
  return `http://${host}:${DEFAULT_API_PORT}${DEFAULT_API_PATH}`;
}

const API_BASE_URL = resolveApiBaseUrl();

function getRequestBaseUrls(): string[] {
  const urls = [API_BASE_URL];

  if (__DEV__) {
    for (const host of getExpoDevHosts()) {
      urls.push(`http://${host}:${DEFAULT_API_PORT}${DEFAULT_API_PATH}`);
    }
  }

  if (__DEV__ && Platform.OS === 'android') {
    urls.push(`http://localhost:${DEFAULT_API_PORT}${DEFAULT_API_PATH}`);
    urls.push(`http://127.0.0.1:${DEFAULT_API_PORT}${DEFAULT_API_PATH}`);
    urls.push(`http://10.0.2.2:${DEFAULT_API_PORT}${DEFAULT_API_PATH}`);
  }

  return Array.from(new Set(urls.map(normalizeBaseUrl)));
}

const API_BASE_URL_CANDIDATES = getRequestBaseUrls();

export const FLUXA_SITE_ORIGINS: Record<FluxASite, string> = {
  paid: 'https://fluxa.camila.qzz.io',
  free: 'https://free.camila.qzz.io',
};

export function resolveFluxASiteOrigin(site: unknown): string {
  if (site === 'paid' || site === 'free') {
    return FLUXA_SITE_ORIGINS[site];
  }

  throw new Error('Unsupported FluxA site');
}

type UnknownRecord = Record<string, unknown>;

function asRecord(value: unknown): UnknownRecord | null {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? value as UnknownRecord
    : null;
}

function asNonEmptyString(value: unknown): string | undefined {
  if (typeof value !== 'string') {
    return undefined;
  }

  const normalized = value.trim();
  return normalized || undefined;
}

function parseFluxAUser(value: unknown): FluxALoginResult['user'] | undefined {
  const user = asRecord(value);
  const id = user?.id;
  const username = asNonEmptyString(user?.username);

  if (!user || typeof id !== 'number' || !Number.isFinite(id) || !username) {
    return undefined;
  }

  const displayName = asNonEmptyString(user.display_name) ?? asNonEmptyString(user.displayName);
  const email = asNonEmptyString(user.email);

  return {
    id,
    username,
    ...(displayName ? {displayName} : {}),
    ...(email ? {email} : {}),
  };
}

export function parseFluxALoginResponse(payload: unknown): FluxALoginResult {
  const response = asRecord(payload);
  const data = asRecord(response?.data);
  const message = asNonEmptyString(response?.message);
  const user = parseFluxAUser(data?.user);
  const result = {
    ...(message ? {message} : {}),
    ...(user ? {user} : {}),
  };

  if (data?.require_2fa === true) {
    return {...result, require2FA: true};
  }

  const accessToken = asNonEmptyString(data?.access_token);
  if (response?.success !== true || !accessToken) {
    return result;
  }

  return {...result, accessToken};
}

async function readErrorMessage(response: Response): Promise<string> {
  const contentType = response.headers.get('content-type') ?? '';

  if (contentType.includes('application/json')) {
    const payload = await response
      .json()
      .catch(() => null) as
      | {error?: unknown; message?: unknown; code?: unknown}
      | null;
    const apiMessage =
      typeof payload?.error === 'string'
        ? payload.error
        : typeof payload?.message === 'string'
          ? payload.message
          : null;
    if (apiMessage && apiMessage.trim()) {
      return apiMessage;
    }
  }

  const rawText = (await response.text().catch(() => '')).trim();
  if (rawText) {
    return rawText;
  }

  return `Request failed (${response.status})`;
}

async function request<T>(path: string, init: RequestInit = {}, token?: string): Promise<T> {
  const requestUrls = API_BASE_URL_CANDIDATES.map((baseUrl) => `${baseUrl}${path}`);
  let response: Response | null = null;
  let lastNetworkError: string | null = null;

  for (const requestUrl of requestUrls) {
    try {
      response = await fetch(requestUrl, {
        ...init,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? {Authorization: `Bearer ${token}`} : {}),
          ...(init.headers ?? {}),
        },
      });
      break;
    } catch (error) {
      lastNetworkError = error instanceof Error ? error.message : 'Unknown network error';
    }
  }

  if (!response) {
    throw new Error(
      `Network error: ${lastNetworkError ?? 'Request failed'} (${requestUrls.join(' | ')})`,
    );
  }

  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

function defaultChatModelTemperature(purpose?: ChatModelPurpose): number {
  switch (purpose) {
    case 'knowledge':
      return 0.05;
    case 'general':
    default:
      return 0.05;
  }
}

function normalizeChatModel(model: UserChatModel): UserChatModel {
  const temperature =
    typeof model.temperature === 'number' && Number.isFinite(model.temperature)
      ? model.temperature
      : defaultChatModelTemperature(model.purpose);
  return {
    ...model,
    temperature,
  };
}

function normalizeChatModelGroups(groups: UserChatModelGroups): UserChatModelGroups {
  return {
    generalModels: (groups.generalModels ?? []).map(normalizeChatModel),
    knowledgeModels: (groups.knowledgeModels ?? []).map(normalizeChatModel),
  };
}

export const api = {
  login: (username: string, password: string) =>
    request<SessionPayload>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({username, password}),
    }),
  loginWithFluxA: async (site: FluxASite, username: string, password: string) => {
    const origin = resolveFluxASiteOrigin(site);
    const response = await fetch(`${origin}/api/user/login`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({username, password}),
    });
    const result = parseFluxALoginResponse(await response.json().catch(() => null));

    if (result.require2FA) {
      return result;
    }

    if (!response.ok || !result.accessToken) {
      throw new Error(result.message ?? 'FluxA login failed');
    }

    return result;
  },
  exchangeFluxASession: (site: FluxASite, accessToken: string) =>
    request<SessionPayload>('/auth/fluxa', {
      method: 'POST',
      body: JSON.stringify({site, accessToken}),
    }),
  loginWithGoogle: (idToken: string) =>
    request<SessionPayload>('/auth/google', {
      method: 'POST',
      body: JSON.stringify({idToken}),
    }),
  loginWithMicrosoft: (idToken: string) =>
    request<SessionPayload>('/auth/microsoft', {
      method: 'POST',
      body: JSON.stringify({idToken}),
    }),
  me: (token: string) => request<User>('/me', {}, token),
  listChatModels: async (token: string) =>
    normalizeChatModelGroups(await request<UserChatModelGroups>('/chat-models', {}, token)),
  createChatModel: (
    token: string,
    payload: {
      purpose: ChatModelPurpose;
      name: string;
      baseUrl: string;
      apiKey?: string;
      modelName: string;
      temperature?: number;
    },
  ) =>
    request<UserChatModel>(
      '/chat-models',
      {
        method: 'POST',
        body: JSON.stringify(payload),
      },
      token,
    ).then(normalizeChatModel),
  updateChatModel: (
    token: string,
    modelId: string,
    payload: {
      name: string;
      baseUrl: string;
      apiKey?: string;
      modelName: string;
      temperature?: number;
    },
  ) =>
    request<UserChatModel>(
      `/chat-models/${modelId}`,
      {
        method: 'PUT',
        body: JSON.stringify(payload),
      },
      token,
    ).then(normalizeChatModel),
  selectChatModel: (token: string, modelId: string) =>
    request<UserChatModel>(
      `/chat-models/${modelId}/select`,
      {
        method: 'POST',
      },
      token,
    ).then(normalizeChatModel),
  deleteChatModel: (token: string, modelId: string) =>
    request<void>(
      `/chat-models/${modelId}`,
      {
        method: 'DELETE',
      },
      token,
    ),
  listRuns: (token: string) =>
    request<{items: Run[]}>('/runs', {}, token),
  createRun: (
    token: string,
    payload: {
      title?: string;
      goal: string;
      mode: 'auto' | 'kb_only' | 'web_only' | 'hybrid';
      knowledge_connection_ids?: string[];
    },
  ) =>
    request<Run>('/runs', {
      method: 'POST',
      body: JSON.stringify(payload),
    }, token),
  getRun: (token: string, runId: string) =>
    request<RunDetails>(`/runs/${runId}`, {}, token),
  listConnections: (token: string) =>
    request<{items: KnowledgeConnection[]}>('/knowledge/connections', {}, token),
  createConnection: (
    token: string,
    payload:
      | {
          provider: 'yuque';
          name: string;
          syncEnabled: boolean;
          config: {
            token: string;
            groupLogin: string;
            namespace?: string;
          };
        }
      | {
          provider: 'feishu';
          name: string;
          syncEnabled: boolean;
          config: {
            appId: string;
            appSecret: string;
            entryType: 'docx' | 'wiki_node' | 'wiki_space';
            entryToken: string;
            documentId?: string;
            documentInput?: string;
            documentUrl?: string;
          };
        },
  ) =>
    request<KnowledgeConnection>('/knowledge/connections', {
      method: 'POST',
      body: JSON.stringify(payload),
    }, token),
  deleteConnection: (token: string, connectionId: string) =>
    request<void>(
      `/knowledge/connections/${connectionId}`,
      {
        method: 'DELETE',
      },
      token,
    ),
  getConnectionDocuments: (token: string, connectionId: string) =>
    request<KnowledgeConnectionDetails>(
      `/knowledge/connections/${connectionId}/documents`,
      {},
      token,
    ),
  syncConnection: (token: string, connectionId: string) =>
    request<KnowledgeSyncJob>(`/knowledge/connections/${connectionId}/sync`, {
      method: 'POST',
    }, token),
  listSyncJobs: (token: string, connectionId: string) =>
    request<{items: KnowledgeSyncJob[]}>(
      `/knowledge/connections/${connectionId}/sync-jobs`,
      {},
      token,
    ),
  listSkills: (token: string) =>
    request<{items: Skill[]}>('/skills', {}, token),
  createSkill: (
    token: string,
    payload: {
      slug?: string;
      title: string;
      description?: string;
      prompt: string;
      mode: ChatSkill;
      source: 'manual' | 'github' | 'upload';
      enabled?: boolean;
      repoUrl?: string;
    },
  ) =>
    request<Skill>(
      '/skills',
      {
        method: 'POST',
        body: JSON.stringify(payload),
      },
      token,
    ),
  importSkillFromGitHub: (
    token: string,
    payload: {
      repoUrl: string;
      ref?: string;
      path?: string;
      install?: boolean;
      enabled?: boolean;
    },
  ) =>
    request<SkillImportResult>(
      '/skill-imports/github',
      {
        method: 'POST',
        body: JSON.stringify(payload),
      },
      token,
    ),
  updateSkill: (
    token: string,
    skillId: string,
    payload: Partial<{
      title: string;
      description: string;
      prompt: string;
      mode: ChatSkill;
      enabled: boolean;
      repoUrl: string;
    }>,
  ) =>
    request<Skill>(
      `/skills/${skillId}`,
      {
        method: 'PATCH',
        body: JSON.stringify(payload),
      },
      token,
    ),
  deleteSkill: (token: string, skillId: string) =>
    request<void>(
      `/skills/${skillId}`,
      {
        method: 'DELETE',
      },
      token,
    ),
  listChatSessions: (token: string) =>
    request<{items: ChatSession[]}>('/chat/sessions', {}, token),
  createChatSession: (token: string, title?: string) =>
    request<ChatSession>(
      '/chat/sessions',
      {
        method: 'POST',
        body: JSON.stringify(title ? {title} : {}),
      },
      token,
    ),
  updateChatSession: (
    token: string,
    sessionId: string,
    payload: Partial<{
      title: string;
      pinned: boolean;
    }>,
  ) =>
    request<ChatSession>(
      `/chat/sessions/${sessionId}`,
      {
        method: 'PATCH',
        body: JSON.stringify(payload),
      },
      token,
    ),
  deleteChatSession: (token: string, sessionId: string) =>
    request<void>(
      `/chat/sessions/${sessionId}`,
      {
        method: 'DELETE',
      },
      token,
    ),
  listChatMessages: (token: string, sessionId: string) =>
    request<{items: PersistedChatMessage[]}>(`/chat/sessions/${sessionId}/messages`, {}, token),
};

export function subscribeRunEvents(
  runId: string,
  accessToken: string,
  onEvent: (event: RunEvent) => void,
) {
  const eventSource = new EventSource(`${API_BASE_URL}/runs/${runId}/events`, {
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  eventSource.addEventListener('message', (event) => {
    if (!event.data) {
      return;
    }
    try {
      const parsed = JSON.parse(event.data) as RunEvent;
      onEvent(parsed);
    } catch (error) {
      console.warn('Failed to parse run event', error);
    }
  });

  return () => {
    eventSource.close();
  };
}

export function subscribeChatStream(
  accessToken: string,
  params: {
    message: string;
    skill: ChatSkill;
    connectionIds?: string[];
    useKnowledge?: boolean;
    skillPrompt?: string;
    skillId?: string;
    sessionId?: string;
    chatModel?: string;
    enableSearch?: boolean;
    temperature?: number;
  },
  onEvent: (event: ChatStreamEvent) => void,
) {
  const query = new URLSearchParams({
    message: params.message,
    skill: params.skill,
    use_knowledge: params.useKnowledge ? 'true' : 'false',
    enable_search: params.enableSearch ? 'true' : 'false',
  });
  if (typeof params.temperature === 'number' && Number.isFinite(params.temperature)) {
    query.set('temperature', String(params.temperature));
  }
  if (params.connectionIds?.length) {
    query.set('connection_ids', params.connectionIds.join(','));
  }
  if (params.skillPrompt?.trim()) {
    query.set('skill_prompt', params.skillPrompt.trim());
  }
  if (params.skillId?.trim()) {
    query.set('skill_id', params.skillId.trim());
  }
  if (params.sessionId?.trim()) {
    query.set('session_id', params.sessionId.trim());
  }
  if (params.chatModel?.trim()) {
    query.set('chat_model', params.chatModel.trim());
  }

  let closed = false;
  const eventSource = new EventSource(`${API_BASE_URL}/chat/stream?${query.toString()}`, {
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  eventSource.addEventListener('message', (event) => {
    if (!event.data) {
      return;
    }
    try {
      const parsed = JSON.parse(event.data) as ChatStreamEvent;
      onEvent(parsed);
    } catch (error) {
      console.warn('Failed to parse chat event', error);
    }
  });

  eventSource.addEventListener('error', (event) => {
    if (closed) {
      return;
    }
    const message =
      'message' in event && typeof event.message === 'string' && event.message.trim()
        ? event.message
        : 'Stream connection failed';
    onEvent({type: 'error', error: message});
    closed = true;
    eventSource.close();
  });

  return () => {
    closed = true;
    eventSource.close();
  };
}

export function useAccessToken() {
  return useAuthStore((state) => state.accessToken);
}

export {API_BASE_URL};
