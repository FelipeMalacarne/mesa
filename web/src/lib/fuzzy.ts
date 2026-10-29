/**
 * Fuzzy matching for quick-open lists, the way fzf and Telescope read a query: its characters
 * must appear in order. An exact name beats a prefix, a prefix beats a hit at a word start,
 * that beats any substring, and a scattered subsequence comes last; within a tier an earlier
 * hit and a shorter name win. `indices` are the characters to highlight.
 */
export interface FuzzyMatch {
  score: number;
  indices: number[];
}

const BREAKS = "_-. /:$";

function wordStart(text: string, i: number): boolean {
  if (i === 0) return true;
  const prev = text[i - 1];
  if (BREAKS.includes(prev)) return true;
  // camelCase: a capital after a lower-case letter.
  const cur = text[i];
  return cur !== cur.toLowerCase() && prev === prev.toLowerCase() && prev !== prev.toUpperCase();
}

/** True when q[k..] appears in order in t[from..]. */
function follows(q: string, k: number, t: string, from: number): boolean {
  for (let i = from; i < t.length && k < q.length; i++) if (t[i] === q[k]) k++;
  return k === q.length;
}

function range(from: number, length: number): number[] {
  return Array.from({ length }, (_, i) => from + i);
}

export function fuzzyMatch(query: string, text: string): FuzzyMatch | null {
  const q = query.toLowerCase();
  const t = text.toLowerCase();
  if (!q) return { score: 0, indices: [] };
  if (q.length > t.length) return null;
  const shorter = -t.length * 0.1;
  if (t === q) return { score: 4000, indices: range(0, q.length) };
  if (t.startsWith(q)) return { score: 3000 + shorter, indices: range(0, q.length) };

  let at = t.indexOf(q);
  if (at >= 0) {
    for (let i = at; i >= 0; i = t.indexOf(q, i + 1)) {
      if (wordStart(text, i)) return { score: 2000 - i + shorter, indices: range(i, q.length) };
    }
    return { score: 1000 - at + shorter, indices: range(at, q.length) };
  }

  // A subsequence: take each character at a later word start when the rest of the query
  // still fits after it, else at its next occurrence.
  const indices: number[] = [];
  let from = 0;
  for (let k = 0; k < q.length; k++) {
    at = t.indexOf(q[k], from);
    if (at < 0) return null;
    for (let j = at; j < t.length; j++) {
      if (t[j] === q[k] && wordStart(text, j)) {
        if (follows(q, k + 1, t, j + 1)) at = j;
        break;
      }
    }
    indices.push(at);
    from = at + 1;
  }
  let gaps = 0;
  let starts = 0;
  for (let k = 0; k < indices.length; k++) {
    if (k > 0 && indices[k] !== indices[k - 1] + 1) gaps++;
    if (wordStart(text, indices[k])) starts++;
  }
  return { score: 500 - gaps * 20 + starts * 15 - indices[0] + shorter, indices };
}
