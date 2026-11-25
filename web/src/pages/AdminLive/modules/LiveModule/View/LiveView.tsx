import { useEffect } from "react";
import { streamLive } from "@/services/admin";
import { LiveRunCard } from "../Component/LiveRunCard";
import { useLive } from "../Model/useLive";

export function LiveView() {
  const live = useLive();
  const { snapshot, event, setConnected } = live;

  useEffect(
    () => streamLive({ onSnapshot: snapshot, onEvent: event, onConnection: setConnected }),
    [snapshot, event, setConnected],
  );

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted" aria-live="polite">
        {live.connected ? "Connected. New runs appear here as they start." : "Connecting… (admins only)"}
      </p>
      {live.runs.length === 0 && (
        <p className="text-muted">No agent runs since core-api started. Search for a product to start one.</p>
      )}
      {live.runs.map((r) => (
        <LiveRunCard key={r.start.run} run={r} />
      ))}
    </div>
  );
}
