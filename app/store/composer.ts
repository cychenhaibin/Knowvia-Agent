import {create} from 'zustand';

import type {RunMode} from '@/types/api';

type ComposerState = {
  title: string;
  goal: string;
  mode: RunMode;
  setTitle: (title: string) => void;
  setGoal: (goal: string) => void;
  setMode: (mode: RunMode) => void;
  reset: () => void;
};

export const useComposerStore = create<ComposerState>((set) => ({
  title: '',
  goal: '',
  mode: 'auto',
  setTitle: (title) => set({title}),
  setGoal: (goal) => set({goal}),
  setMode: (mode) => set({mode}),
  reset: () =>
    set({
      title: '',
      goal: '',
      mode: 'auto',
    }),
}));
