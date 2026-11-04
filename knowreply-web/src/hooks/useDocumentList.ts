import { useCallback, useEffect, useState } from 'react';
import { App as AntdApp } from 'antd';
import { POLL_INTERVAL_MS } from '../config';
import { useAuth } from '../context/AuthContext';
import { listDocuments } from '../services/knowledgeApi';
import { isUnauthorized, toErrorMessage } from '../services/http';
import type { DocItem } from '../types';

interface DocumentListState {
  docs: DocItem[];
  loading: boolean;
  reload: () => void;
}

/**
 * 文档列表数据源：按状态筛选，存在 pending/processing 时自动轮询，落定即停。
 */
export function useDocumentList(statusFilter: string): DocumentListState {
  const { settings, token, clearToken } = useAuth();
  const { message } = AntdApp.useApp();

  const [docs, setDocs] = useState<DocItem[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (silent: boolean) => {
      if (!token) {
        setDocs([]);
        return;
      }
      if (!silent) setLoading(true);
      try {
        const res = await listDocuments(settings, token, statusFilter);
        setDocs(res.items);
      } catch (err) {
        if (isUnauthorized(err)) {
          clearToken();
        } else if (!silent) {
          message.error(`加载文档失败：${toErrorMessage(err)}`);
        }
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [settings, token, statusFilter, clearToken, message],
  );

  useEffect(() => {
    void load(false);
  }, [load]);

  const hasPending = docs.some((d) => d.status === 'pending' || d.status === 'processing');
  useEffect(() => {
    if (!hasPending) return;
    const timer = window.setTimeout(() => {
      void load(true);
    }, POLL_INTERVAL_MS);
    return () => window.clearTimeout(timer);
  }, [hasPending, docs, load]);

  const reload = useCallback(() => {
    void load(false);
  }, [load]);

  return { docs, loading, reload };
}
