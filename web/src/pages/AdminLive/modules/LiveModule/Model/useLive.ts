import { useCallback } from "react";
import type { LiveEvent, LiveRun } from "@/services/admin";
import { useLiveStore } from "../../../store";
import { applyEvent } from "./applyEvent";

export function useLive() {
  const runs = useLiveStore((s) => s.runs);
  const connected = useLiveStore((s) => s.connected);
  const setRuns = useLiveStore((s) => s.setRuns);
  const setConnected = useLiveStore((s) => s.setConnected);
  // Stable: the stream effect depends on them.
  const snapshot = useCallback((next: LiveRun[]) => setRuns(next), [setRuns]);
  const event = useCallback((e: LiveEvent) => setRuns((prev) => applyEvent(prev, e)), [setRuns]);
  return { runs, connected, snapshot, event, setConnected };
}
