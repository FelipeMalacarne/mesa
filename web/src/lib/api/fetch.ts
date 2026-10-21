/**
 * The fetch mutator orval wires into every generated operation.
 *
 * - Prefixes `/api` (the server's base path) unless the URL is absolute.
 * - Sends and expects JSON.
 * - Turns every non-2xx response into an `ApiError` built from the `Error` schema
 *   (`{ kind, message, confirmWith }`), so callers can switch on `kind`.
 *
 * This is the only place `any`-like loosening happens at the HTTP boundary.
 */
import type { ErrorKind } from "./mesaAPI.schemas";

export class ApiError extends Error {
  readonly kind: ErrorKind;
  readonly status: number;
  readonly confirmWith?: string;

  constructor(status: number, kind: ErrorKind, message: string, confirmWith?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.kind = kind;
    if (confirmWith !== undefined) this.confirmWith = confirmWith;
  }
}

export function isApiError(e: unknown): e is ApiError {
  return e instanceof ApiError;
}

/** A readable message for any thrown value (ApiError, Error or something else). */
export function errorMessage(e: unknown): string {
  if (isApiError(e)) return e.message;
  if (e instanceof Error) return e.message;
  return String(e);
}

const API_BASE = "/api";

const KINDS: ReadonlySet<string> = new Set([
  "not_found",
  "invalid",
  "forbidden",
  "conflict",
  "unavailable",
  "unsupported",
  "needs_confirmation",
  "unknown",
]);

function statusKind(status: number): ErrorKind {
  switch (status) {
    case 400:
    case 422:
      return "invalid";
    case 403:
      return "forbidden";
    case 404:
      return "not_found";
    case 409:
      return "conflict";
    case 501:
      return "unsupported";
    case 502:
    case 503:
    case 504:
      return "unavailable";
    default:
      return "unknown";
  }
}

async function toApiError(res: Response): Promise<ApiError> {
  let body: unknown;
  try {
    body = await res.json();
  } catch {
    body = undefined;
  }
  if (body && typeof body === "object") {
    const b = body as { kind?: unknown; message?: unknown; confirmWith?: unknown };
    const kind = typeof b.kind === "string" && KINDS.has(b.kind) ? (b.kind as ErrorKind) : statusKind(res.status);
    const message = typeof b.message === "string" && b.message ? b.message : res.statusText || `Request failed (${res.status})`;
    const confirmWith = typeof b.confirmWith === "string" ? b.confirmWith : undefined;
    return new ApiError(res.status, kind, message, confirmWith);
  }
  return new ApiError(res.status, statusKind(res.status), res.statusText || `Request failed (${res.status})`);
}

export const customFetch = async <T>(url: string, options: RequestInit = {}): Promise<T> => {
  const fullUrl = /^https?:\/\//.test(url) ? url : `${API_BASE}${url}`;
  const headers = new Headers(options.headers);
  headers.set("Accept", "application/json");
  if (options.body !== undefined && options.body !== null && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  let res: Response;
  try {
    res = await fetch(fullUrl, { ...options, headers });
  } catch (e) {
    throw new ApiError(0, "unavailable", e instanceof Error ? e.message : "Network error");
  }

  if (!res.ok) throw await toApiError(res);

  if (res.status === 204 || res.headers.get("content-length") === "0") {
    return undefined as T;
  }
  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
};

export default customFetch;
