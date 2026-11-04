import { useCallback, useEffect, useState } from 'react';
import { App as AntdApp } from 'antd';
import { POLL_INTERVAL_MS } from '../config';
import { useAuth } from '../context/AuthContext';
import { getDocumentStats, listDocuments } from '../services/knowledgeApi';
import { isUnauthorized, toErrorMessage } from '../services/http';
import type { DocItem, DocStats } from '../types';

interface DocumentListState {
  docs: DocItem[];
  stats: DocStats | null;
  loading: boolean;
  reload: () => void;
}

/**
 * 文档列表 + 汇总数据源。
 * 列表与统计并发拉取；存在 pending/processing 时自动轮询，落定即停。
 */
export function useDocumentList(statusFilter: string): DocumentListState {
  const { settings, token, clearToken } = useAuth();
  const { message } = AntdApp.useApp();

  const [docs, setDocs] = useState<DocItem[]>([]);
  const [stats, setStats] = useState<DocStats | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(
    async (silent: boolean) => {
      if (!token) {
        setDocs([]);
        setStats(null);
        return;
      }
      if (!silent) setLoading(true);
      try {
        const [list, stat] = await Promise.all([
          listDocuments(settings, token, statusFilter),
          getDocumentStats(settings, token),
        ]);
        setDocs(list.items);
        setStats(stat);
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

  return { docs, stats, loading, reload };
}
