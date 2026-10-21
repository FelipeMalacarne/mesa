/**
 * What the active view wants shown in the status bar after the connection status: short facts
 * ("1,284 rows", "42 ms", "7 columns · 3 indexes"), page buttons, and staged changes waiting for
 * review. Views call `useStatusFacts(facts, paging)` and `usePendingStatus(pending)`.
 */
import { useEffect, useRef } from "react";
import { create } from "zustand";

export interface Paging {
  page: number;
  pages: number;
  onPrev: () => void;
  onNext: () => void;
}

export interface Pending {
  summary: string;
  onDiscard: () => void;
  onReview: () => void;
}

interface StatusState {
  facts: string[];
  paging: Paging | null;
  pending: Pending | null;
  set: (facts: string[], paging?: Paging | null) => void;
  setPending: (pending: Pending | null) => void;
  clear: () => void;
}

export const useStatus = create<StatusState>((set) => ({
  facts: [],
  paging: null,
  pending: null,
  set: (facts, paging = null) => set({ facts, paging }),
  setPending: (pending) => set({ pending }),
  clear: () => set({ facts: [], paging: null, pending: null }),
}));

/** Publishes facts for as long as the calling view is mounted. */
export function useStatusFacts(facts: string[], paging?: Paging | null): void {
  const set = useStatus((s) => s.set);
  const clear = useStatus((s) => s.clear);
  const pagingRef = useRef(paging);
  pagingRef.current = paging;
  const key = facts.join("\u0000");
  const page = paging?.page ?? 0;
  const pages = paging?.pages ?? 0;
  useEffect(() => {
    const p = pagingRef.current;
    set(
      facts,
      p
        ? {
            page: p.page,
            pages: p.pages,
            onPrev: () => pagingRef.current?.onPrev(),
            onNext: () => pagingRef.current?.onNext(),
          }
        : null,
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, page, pages, set]);
  useEffect(() => () => clear(), [clear]);
}

/** Shows staged changes in the status bar while the view is mounted. */
export function usePendingStatus(pending: Pending | null): void {
  const setPending = useStatus((s) => s.setPending);
  const ref = useRef(pending);
  ref.current = pending;
  const summary = pending?.summary ?? "";
  useEffect(() => {
    if (!summary) {
      setPending(null);
      return;
    }
    setPending({ summary, onDiscard: () => ref.current?.onDiscard(), onReview: () => ref.current?.onReview() });
  }, [summary, setPending]);
  useEffect(() => () => setPending(null), [setPending]);
}
