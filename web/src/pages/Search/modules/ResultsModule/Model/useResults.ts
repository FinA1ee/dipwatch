import { useCallback } from "react";
import type { DoneEvent, OffersEvent } from "@/services/api";
import { useSearchStore } from "../../../store";
import { filterOffers } from "./filterOffers";

/**
 * Results for display, and the stream's updates into the store. The handlers are
 * stable: the stream effect depends on them, and a new identity per render would
 * close and reopen the stream on every event.
 */
export function useResults() {
  const phase = useSearchStore((s) => s.phase);
  const product = useSearchStore((s) => s.product);
  const offers = useSearchStore((s) => s.offers);
  const pending = useSearchStore((s) => s.pending);
  const error = useSearchStore((s) => s.error);
  const timedOut = useSearchStore((s) => s.timedOut);
  const paused = useSearchStore((s) => s.paused);
  const filters = useSearchStore((s) => s.filters);
  const patch = useSearchStore((s) => s.patch);
  // Each event carries the whole ranked list, so it replaces the current one.
  const applyOffers = useCallback(
    (e: OffersEvent) => patch({ offers: e.offers, pending: e.pending }),
    [patch],
  );
  const finish = useCallback(
    (e: DoneEvent) => patch({ phase: "done", pending: [], timedOut: e.timed_out }),
    [patch],
  );

  return {
    phase,
    product,
    error,
    timedOut,
    paused,
    pending,
    total: offers.length,
    shown: filterOffers(offers, filters),
    applyOffers,
    finish,
  };
}
