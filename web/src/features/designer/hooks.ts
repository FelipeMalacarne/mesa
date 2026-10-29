/** Small hooks the designer shares. */
import * as React from "react";

/** A per-browser preference in localStorage. */
export function usePref<T extends string>(key: string, initial: T, allowed: readonly T[]): [T, (v: T) => void] {
  const [v, setV] = React.useState<T>(() => {
    try {
      const s = localStorage.getItem(key) as T | null;
      return s && allowed.includes(s) ? s : initial;
    } catch {
      return initial;
    }
  });
  const set = React.useCallback(
    (next: T) => {
      setV(next);
      try {
        localStorage.setItem(key, next);
      } catch {
        // Blocked storage: the choice lasts for this page.
      }
    },
    [key],
  );
  return [v, set];
}

/** The value once it has stopped changing for `ms`. */
export function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = React.useState(value);
  React.useEffect(() => {
    const t = window.setTimeout(() => setV(value), ms);
    return () => window.clearTimeout(t);
  }, [value, ms]);
  return v;
}
