export type DocStatus = 'pending' | 'processing' | 'ready' | 'failed' | 'offline';

export interface DocItem {
  id: number;
  tenantId: number;
  title: string;
  sourceUrl: string | null;
  /** 源文件字节数 */
  sourceSize: number;
  status: DocStatus;
  /** 写入 pgvector 的切片数 */
  chunkCount: number;
  /** 最近一次失败原因，成功时为空 */
  errorMsg: string | null;
  version: string;
  effectiveAt: string | null;
  createdAt: string | null;
  updatedAt: string | null;
}

/** 知识库汇总，管理台顶部概览用 */
export interface DocStats {
  total: number;
  pending: number;
  processing: number;
  ready: number;
  failed: number;
  offline: number;
  chunks: number;
  bytes: number;
}

export interface DocListResponse {
  items: DocItem[];
  limit: number;
  offset: number;
}

export interface Hit {
  chunkId: number;
  docId: number;
  headingPath: string;
  content: string;
  vectorScore: number;
  textScore: number;
  gramScore: number;
  fusedScore: number;
  vectorRank: number;
  textRank: number;
  gramRank: number;
}

export interface SearchResponse {
  query: string;
  vectorHits: number;
  textHits: number;
  gramHits: number;
  embedder: string;
  tookMs: number;
  hits: Hit[];
}

export interface TokenResponse {
  token: string;
  expiresAt: string;
  expiresIn: number;
}

export interface UploadResponse {
  docId: number;
  taskId: string;
  title: string;
  status: DocStatus;
}

export interface Settings {
  apiBase: string;
  kbBase: string;
  tenantId: number;
  userId: string;
}

export const STATUS_TEXT: Record<DocStatus, string> = {
  pending: '待处理',
  processing: '处理中',
  ready: '已入库',
  failed: '失败',
  offline: '已下线',
};

export const STATUS_COLOR: Record<DocStatus, string> = {
  pending: 'gold',
  processing: 'blue',
  ready: 'green',
  failed: 'red',
  offline: 'default',
};
