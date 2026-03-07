export const customInstance = async <T>(
  url: string,
  options?: RequestInit
): Promise<T> => {
  const base = import.meta.env.VITE_API_BASE ?? "/api";
  const fullUrl = `${base}${url}`;

  const res = await fetch(fullUrl, options);

  if (!res.ok) {
    const error = await res.json().catch(() => ({ message: res.statusText }));
    throw error;
  }

  if (res.status === 204 || res.headers.get("content-length") === "0") {
    return undefined as T;
  }

  return res.json();
};
