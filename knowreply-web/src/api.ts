import type {
  DocListResponse,
  SearchResponse,
  Settings,
  TokenResponse,
  UploadResponse,
} from './types';

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

function join(base: string, path: string): string {
  return base.replace(/\/+$/, '') + path;
}

async function parse<T>(res: Response): Promise<T> {
  const text = await res.text();
  let body: unknown = null;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    body = { error: text };
  }

  if (!res.ok) {
    const detail =
      (body as { error?: string; message?: string } | null)?.error ??
      (body as { message?: string } | null)?.message ??
      res.statusText;
    throw new ApiError(res.status, detail);
  }
  return body as T;
}

export async function issueToken(settings: Settings): Promise<TokenResponse> {
  const res = await fetch(join(settings.apiBase, '/api/v1/auth/token'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tenantId: settings.tenantId, userId: settings.userId }),
  });
  return parse<TokenResponse>(res);
}

export async function listDocuments(
  settings: Settings,
  token: string,
  status?: string,
): Promise<DocListResponse> {
  const query = new URLSearchParams({ limit: '50' });
  if (status) query.set('status', status);

  const res = await fetch(join(settings.apiBase, `/api/v1/kb/documents?${query}`), {
    headers: { Authorization: `Bearer ${token}` },
  });
  return parse<DocListResponse>(res);
}

export async function uploadDocument(
  settings: Settings,
  token: string,
  file: File,
  title?: string,
): Promise<UploadResponse> {
  const form = new FormData();
  form.append('file', file);
  if (title) form.append('title', title);

  const res = await fetch(join(settings.apiBase, '/api/v1/kb/documents'), {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });
  return parse<UploadResponse>(res);
}

export async function offlineDocument(
  settings: Settings,
  token: string,
  docId: number,
): Promise<{ docId: number; status: string }> {
  const res = await fetch(join(settings.apiBase, `/api/v1/kb/documents/${docId}`), {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
  });
  return parse<{ docId: number; status: string }>(res);
}

export async function searchChunks(
  settings: Settings,
  query: string,
  limit: number,
): Promise<SearchResponse> {
  const res = await fetch(join(settings.kbBase, '/internal/kb/search'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tenantId: settings.tenantId, query, limit }),
  });
  return parse<SearchResponse>(res);
}
