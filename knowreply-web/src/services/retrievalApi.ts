import type { SearchResponse, Settings } from '../types';
import { requestJson, withBase } from './http';

/**
 * 检索预览：调 worker 侧的内部接口，三路召回 + RRF 融合。
 * 正式形态由 Node Agent 承担检索编排，这里保留给运营做质量排查。
 */
export function searchChunks(
  settings: Settings,
  query: string,
  limit: number,
): Promise<SearchResponse> {
  return requestJson<SearchResponse>(withBase(settings.kbBase, '/internal/kb/search'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tenantId: settings.tenantId, query, limit }),
  });
}
