/** 统一的 HTTP 封装：错误归一到 ApiError，调用方只关心 message 与 status。 */
export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export function withBase(base: string, path: string): string {
  return base.replace(/\/+$/, '') + path;
}

export function bearer(token: string): Record<string, string> {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export async function requestJson<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  const text = await res.text();

  let body: unknown = null;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    body = { error: text };
  }

  if (!res.ok) {
    const payload = body as { error?: string; message?: string } | null;
    throw new ApiError(res.status, payload?.error ?? payload?.message ?? res.statusText);
  }
  return body as T;
}

export function isUnauthorized(err: unknown): boolean {
  return err instanceof ApiError && err.status === 401;
}

export function toErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
