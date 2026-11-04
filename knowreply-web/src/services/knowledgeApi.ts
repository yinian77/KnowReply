import { DOC_PAGE_SIZE } from '../config';
import type { DocListResponse, Settings, TokenResponse, UploadResponse } from '../types';
import { bearer, requestJson, withBase } from './http';

/** 签发访问令牌（联调用简洁登录，正式环境替换为账号体系） */
export function issueToken(settings: Settings): Promise<TokenResponse> {
  return requestJson<TokenResponse>(withBase(settings.apiBase, '/api/v1/auth/token'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tenantId: settings.tenantId, userId: settings.userId }),
  });
}

export function listDocuments(
  settings: Settings,
  token: string,
  status?: string,
): Promise<DocListResponse> {
  const query = new URLSearchParams({ limit: String(DOC_PAGE_SIZE) });
  if (status) query.set('status', status);

  return requestJson<DocListResponse>(
    withBase(settings.apiBase, `/api/v1/kb/documents?${query.toString()}`),
    { headers: bearer(token) },
  );
}

export function uploadDocument(
  settings: Settings,
  token: string,
  file: File,
  title?: string,
): Promise<UploadResponse> {
  const form = new FormData();
  form.append('file', file);
  if (title) form.append('title', title);

  return requestJson<UploadResponse>(withBase(settings.apiBase, '/api/v1/kb/documents'), {
    method: 'POST',
    headers: bearer(token),
    body: form,
  });
}

export function offlineDocument(
  settings: Settings,
  token: string,
  docId: number,
): Promise<{ docId: number; status: string }> {
  return requestJson<{ docId: number; status: string }>(
    withBase(settings.apiBase, `/api/v1/kb/documents/${docId}`),
    { method: 'DELETE', headers: bearer(token) },
  );
}
