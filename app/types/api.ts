export type RunMode = 'auto' | 'kb_only' | 'web_only' | 'hybrid';
export type RunStatus = 'queued' | 'planning' | 'running' | 'completed' | 'failed';
export type StepStatus = 'pending' | 'running' | 'completed' | 'failed';
export type StepKind =
  | 'planning'
  | 'yuque_search'
  | 'web_search'
  | 'web_page_extract'
  | 'evidence_merge'
  | 'report_writer'
  | 'finalize';

export interface User {
  id: string;
  username: string;
  displayName: string;
  email?: string;
  avatarUrl?: string;
}

export type ChatModelPurpose = 'general' | 'knowledge';
export type ChatModelOrigin = 'default' | 'custom';

export interface UserChatModel {
  id: string;
  userId: string;
  purpose: ChatModelPurpose;
  origin: ChatModelOrigin;
  name: string;
  baseUrl: string;
  apiKey?: string;
  modelName: string;
  temperature: number;
  isSelected: boolean;
  available: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface UserChatModelGroups {
  generalModels: UserChatModel[];
  knowledgeModels: UserChatModel[];
}

export interface SessionPayload {
  user: User;
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

export type FluxASite = 'paid' | 'free';

export type FluxALoginResult = {
  accessToken?: string;
  require2FA?: boolean;
  message?: string;
  user?: {id: number; username: string; displayName?: string; email?: string};
};

export interface Run {
  id: string;
  userId: string;
  title: string;
  goal: string;
  requestedMode: RunMode;
  effectiveMode: RunMode;
  knowledgeConnectionIds?: string[];
  status: RunStatus;
  latestArtifactId?: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
}

export interface RunStep {
  id: string;
  runId: string;
  kind: StepKind;
  label: string;
  status: StepStatus;
  summary: string;
  startedAt?: string;
  finishedAt?: string;
  createdAt: string;
}

export interface RunArtifact {
  id: string;
  runId: string;
  kind: 'report_outline' | 'report_draft' | 'report_grounding' | 'report' | 'final_answer';
  contentMarkdown: string;
  version: number;
  createdAt: string;
}

export interface RunSource {
  id: string;
  runId: string;
  provider: 'yuque' | 'feishu' | 'web';
  connectionId?: string;
  documentId?: string;
  chunkId?: string;
  title: string;
  repo?: string;
  url?: string;
  snippet: string;
  score: number;
  createdAt: string;
}

export interface RunDetails {
  run: Run;
  steps: RunStep[];
  artifacts: RunArtifact[];
  sources: RunSource[];
}

export interface RunEvent {
  type:
    | 'run.created'
    | 'run.planned'
    | 'step.started'
    | 'step.updated'
    | 'step.completed'
    | 'artifact.updated'
    | 'run.completed'
    | 'run.failed';
  runId: string;
  timestamp: string;
  payload?: Record<string, unknown>;
}

export interface KnowledgeConnection {
  id: string;
  userId: string;
  provider: 'yuque' | 'feishu';
  name: string;
  syncEnabled: boolean;
  lastSyncedAt?: string;
  yuque?: {
    groupLogin: string;
    namespace?: string;
  };
  feishu?: {
    appId: string;
    entryType: 'docx' | 'wiki_node' | 'wiki_space';
    entryToken: string;
  };
  createdAt: string;
  updatedAt: string;
}

export interface KnowledgeSyncJob {
  id: string;
  userId: string;
  connectionId: string;
  status: 'queued' | 'running' | 'completed' | 'failed';
  summary: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
}

export interface KnowledgeRepoSummary {
  name: string;
  documentCount: number;
}

export interface KnowledgeDocumentSummary {
  id: string;
  userId: string;
  connectionId: string;
  repo: string;
  title: string;
  docRef: string;
  sourceUrl?: string;
  bodyHash?: string;
  chunkCount?: number;
  updatedAt: string;
  createdAt: string;
}

export interface KnowledgeConnectionDetails {
  connection: KnowledgeConnection;
  repoCount: number;
  documentCount: number;
  chunkCount: number;
  repos: KnowledgeRepoSummary[];
  documents: KnowledgeDocumentSummary[];
}

export type ChatSkill = 'answer' | 'summary' | 'actions';
export type SkillSource = 'manual' | 'github' | 'upload';

export interface Skill {
  id: string;
  userId: string;
  slug: string;
  title: string;
  description: string;
  prompt: string;
  mode: ChatSkill;
  source: SkillSource;
  enabled: boolean;
  repoUrl?: string;
  createdAt: string;
  updatedAt: string;
}

export type SkillImportStatus = 'pending' | 'completed' | 'failed';

export interface SkillArtifact {
  id: string;
  userId: string;
  definitionId?: string;
  revisionId?: string;
  source: SkillSource;
  fileName: string;
  mediaType?: string;
  sourceUrl?: string;
  sha256: string;
  sizeBytes: number;
  entryPath?: string;
  manifestPath?: string;
  instructionsPath?: string;
  createdAt: string;
}

export interface SkillImportJob {
  id: string;
  userId: string;
  source: SkillSource;
  status: SkillImportStatus;
  artifactId?: string;
  definitionId?: string;
  revisionId?: string;
  installationId?: string;
  errorMessage?: string;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

export interface SkillImportResult {
  job: SkillImportJob;
  artifact?: SkillArtifact;
}

export interface ChatSession {
  id: string;
  userId: string;
  title: string;
  pinned: boolean;
  lastMessageAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ChatSource {
  provider: 'yuque' | 'feishu' | 'web';
  title: string;
  repo?: string;
  url?: string;
  snippet: string;
  matched_answer_lines?: string[];
  score: number;
}

export interface ChatUsage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

export interface PersistedChatMessage {
  id: string;
  sessionId: string;
  role: 'user' | 'assistant';
  content: string;
  skill?: string;
  useKnowledge: boolean;
  createdAt: string;
  completedAt?: string;
  sources?: ChatSource[];
  usage?: ChatUsage;
}

export interface ChatStreamEvent {
  type: 'session' | 'retrieval' | 'chunk' | 'done' | 'error';
  content?: string;
  skill?: ChatSkill;
  error?: string;
  sources?: ChatSource[];
  usage?: ChatUsage;
  session_id?: string;
  title?: string;
}
