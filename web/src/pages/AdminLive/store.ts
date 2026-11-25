import { create } from "zustand";
import type { LiveRun } from "@/services/admin";

type Update<T> = T | ((prev: T) => T);

type LiveState = {
  runs: LiveRun[]; // newest first
  connected: boolean;
  setRuns: (next: Update<LiveRun[]>) => void;
  setConnected: (connected: boolean) => void;
};

/** Live agent runs for the admin page. */
export const useLiveStore = create<LiveState>()((set) => ({
  runs: [],
  connected: false,
  setRuns: (next) => set((s) => ({ runs: typeof next === "function" ? next(s.runs) : next })),
  setConnected: (connected) => set({ connected }),
}));
