import {create} from 'zustand';

import {api} from '@/lib/api';
import {queryClient} from '@/lib/query-client';
import {useAuthStore} from '@/store/auth';
import type {ChatSkill, Skill, SkillSource} from '@/types/api';

export type InstalledSkill = Skill;
export type {SkillSource};

type SkillsState = {
  installedSkills: InstalledSkill[];
  bootstrapped: boolean;
  bootstrap: () => Promise<void>;
  refresh: () => Promise<void>;
  addSkill: (skill: {
    slug?: string;
    title: string;
    description?: string;
    prompt: string;
    mode: ChatSkill;
    source: SkillSource;
    enabled?: boolean;
    repoUrl?: string;
  }) => Promise<void>;
  importGitHubSkill: (payload: {
    repoUrl: string;
    ref?: string;
    path?: string;
  }) => Promise<void>;
  updateSkill: (
    id: string,
    patch: Partial<{
      title: string;
      description: string;
      prompt: string;
      mode: ChatSkill;
      enabled: boolean;
      repoUrl: string;
    }>,
  ) => Promise<void>;
  removeSkill: (id: string) => Promise<void>;
  toggleSkill: (id: string) => Promise<void>;
};

async function fetchSkills() {
  const token = useAuthStore.getState().accessToken;
  if (!token) {
    return [];
  }
  const response = await api.listSkills(token);
  return response.items;
}

export const useSkillsStore = create<SkillsState>((set, get) => ({
  installedSkills: [],
  bootstrapped: false,
  bootstrap: async () => {
    try {
      const installedSkills = await fetchSkills();
      set({installedSkills, bootstrapped: true});
    } catch {
      set({installedSkills: [], bootstrapped: true});
    }
  },
  refresh: async () => {
    const installedSkills = await fetchSkills();
    set({installedSkills});
    queryClient.invalidateQueries({queryKey: ['skills']}).catch(() => {});
  },
  addSkill: async (skill) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return;
    }
    const created = await api.createSkill(token, {
      slug: skill.slug,
      title: skill.title,
      description: skill.description,
      prompt: skill.prompt,
      mode: skill.mode,
      source: skill.source,
      enabled: skill.enabled,
      repoUrl: skill.repoUrl,
    });
    set({installedSkills: [created, ...get().installedSkills]});
    queryClient.invalidateQueries({queryKey: ['skills']}).catch(() => {});
  },
  importGitHubSkill: async ({repoUrl, ref, path}) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return;
    }
    await api.importSkillFromGitHub(token, {
      repoUrl,
      ref,
      path,
      install: true,
      enabled: true,
    });
    const installedSkills = await fetchSkills();
    set({installedSkills});
    queryClient.invalidateQueries({queryKey: ['skills']}).catch(() => {});
  },
  updateSkill: async (id, patch) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return;
    }
    const updated = await api.updateSkill(token, id, patch);
    set({
      installedSkills: get().installedSkills.map((item) => (item.id === id ? updated : item)),
    });
    queryClient.invalidateQueries({queryKey: ['skills']}).catch(() => {});
  },
  removeSkill: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return;
    }
    await api.deleteSkill(token, id);
    set({
      installedSkills: get().installedSkills.filter((item) => item.id !== id),
    });
    queryClient.invalidateQueries({queryKey: ['skills']}).catch(() => {});
  },
  toggleSkill: async (id) => {
    const token = useAuthStore.getState().accessToken;
    if (!token) {
      return;
    }
    const current = get().installedSkills.find((item) => item.id === id);
    if (!current) {
      return;
    }
    const nextEnabled = !current.enabled;

    set({
      installedSkills: get().installedSkills.map((item) =>
        item.id === id ? {...item, enabled: nextEnabled} : item,
      ),
    });

    try {
      const updated = await api.updateSkill(token, id, {enabled: nextEnabled});
      set({
        installedSkills: get().installedSkills.map((item) => (item.id === id ? updated : item)),
      });
      queryClient.invalidateQueries({queryKey: ['skills']}).catch(() => {});
    } catch (error) {
      set({
        installedSkills: get().installedSkills.map((item) =>
          item.id === id ? {...item, enabled: current.enabled} : item,
        ),
      });
      throw error;
    }
  },
}));
