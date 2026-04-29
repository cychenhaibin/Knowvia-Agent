import type {ChatSkill, ChatSource, ChatUsage} from '@/types/api';

export type KnowledgeMode = 'none' | 'all' | 'selected';

export type ChatMessage = {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  sources?: ChatSource[];
  usage?: ChatUsage;
  state?: 'streaming' | 'error' | 'done';
};

export type SkillChoice = {
  id: string;
  title: string;
  prompt: string;
  mode: ChatSkill;
};

export type ModelChoice = {
  id: string;
  modelName: string;
  title: string;
  description: string;
  temperature: number;
  available?: boolean;
};

export type MenuPosition = {
  top: number;
  left: number;
  width: number;
};

export type MenuAnchorRect = {
  top: number;
  left: number;
  width: number;
  height: number;
};
