import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { App as AntdApp } from 'antd';
import { DEFAULT_SETTINGS, STORAGE_KEYS } from '../config';
import { issueToken } from '../services/knowledgeApi';
import { toErrorMessage } from '../services/http';
import type { Settings } from '../types';

interface AuthContextValue {
  settings: Settings;
  token: string;
  connected: boolean;
  connecting: boolean;
  /** 连接（签发令牌），成功返回 true */
  connect: (values: Settings) => Promise<boolean>;
  disconnect: () => void;
  /** 令牌失效时由请求层调用 */
  clearToken: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function readSettings(): Settings {
  try {
    const raw = localStorage.getItem(STORAGE_KEYS.settings);
    if (!raw) return DEFAULT_SETTINGS;
    return { ...DEFAULT_SETTINGS, ...(JSON.parse(raw) as Partial<Settings>) };
  } catch {
    return DEFAULT_SETTINGS;
  }
}

function readToken(): string {
  return localStorage.getItem(STORAGE_KEYS.token) ?? '';
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const { message } = AntdApp.useApp();
  const [settings, setSettings] = useState<Settings>(readSettings);
  const [token, setToken] = useState<string>(readToken);
  const [connecting, setConnecting] = useState(false);

  const connect = useCallback(
    async (values: Settings): Promise<boolean> => {
      setConnecting(true);
      try {
        const res = await issueToken(values);
        localStorage.setItem(STORAGE_KEYS.settings, JSON.stringify(values));
        localStorage.setItem(STORAGE_KEYS.token, res.token);
        setSettings(values);
        setToken(res.token);
        message.success(
          `已连接租户 ${values.tenantId}，令牌有效期至 ${new Date(res.expiresAt).toLocaleString()}`,
        );
        return true;
      } catch (err) {
        message.error(`获取令牌失败：${toErrorMessage(err)}`);
        return false;
      } finally {
        setConnecting(false);
      }
    },
    [message],
  );

  const disconnect = useCallback(() => {
    localStorage.removeItem(STORAGE_KEYS.token);
    setToken('');
  }, []);

  const clearToken = useCallback(() => {
    localStorage.removeItem(STORAGE_KEYS.token);
    setToken('');
    message.warning('令牌已失效，请重新获取');
  }, [message]);

  const value = useMemo<AuthContextValue>(
    () => ({
      settings,
      token,
      connected: Boolean(token),
      connecting,
      connect,
      disconnect,
      clearToken,
    }),
    [settings, token, connecting, connect, disconnect, clearToken],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth 必须在 AuthProvider 内使用');
  return ctx;
}
