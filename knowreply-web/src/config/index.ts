import type { Settings } from '../types';

export const STORAGE_KEYS = {
  settings: 'kr_settings',
  token: 'kr_token',
} as const;

export const DEFAULT_SETTINGS: Settings = {
  apiBase: 'http://127.0.0.1:8080',
  kbBase: 'http://127.0.0.1:8081',
  tenantId: 1,
  userId: 'admin',
};

/** 文档列表单页条数 */
export const DOC_PAGE_SIZE = 50;

/** 有在途任务时的轮询间隔 */
export const POLL_INTERVAL_MS = 3000;

/** 与后端网关、worker 解析器保持一致 */
export const ACCEPTED_EXTENSIONS = '.md,.markdown,.txt,.docx,.pdf';
export const MAX_UPLOAD_MB = 50;

export const PATHS = {
  root: '/',
  knowledge: '/knowledge',
  retrieval: '/retrieval',
} as const;
