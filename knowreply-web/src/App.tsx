import { useCallback, useEffect, useState } from 'react';
import { App as AntdApp, Button, Layout, Space, Tabs, Tag, Typography } from 'antd';
import { ApiError, issueToken, listDocuments } from './api';
import DocTable from './components/DocTable';
import SearchPanel from './components/SearchPanel';
import SettingsModal from './components/SettingsModal';
import UploadPanel from './components/UploadPanel';
import { DEFAULT_SETTINGS } from './types';
import type { DocItem, Settings } from './types';

const { Header, Content } = Layout;
const { Title } = Typography;

const LS_SETTINGS = 'kr_settings';
const LS_TOKEN = 'kr_token';
const POLL_INTERVAL = 3000;

function loadSettings(): Settings {
  try {
    const raw = localStorage.getItem(LS_SETTINGS);
    if (!raw) return DEFAULT_SETTINGS;
    return { ...DEFAULT_SETTINGS, ...(JSON.parse(raw) as Partial<Settings>) };
  } catch {
    return DEFAULT_SETTINGS;
  }
}

export default function App() {
  const { message } = AntdApp.useApp();

  const [settings, setSettings] = useState<Settings>(loadSettings);
  const [token, setToken] = useState<string>(() => localStorage.getItem(LS_TOKEN) ?? '');
  const [docs, setDocs] = useState<DocItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [statusFilter, setStatusFilter] = useState('');
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [connecting, setConnecting] = useState(false);

  const loadDocs = useCallback(
    async (target: Settings, authToken: string, status: string, silent = false) => {
      if (!authToken) {
        setDocs([]);
        return;
      }
      if (!silent) setLoading(true);
      try {
        const res = await listDocuments(target, authToken, status);
        setDocs(res.items);
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          localStorage.removeItem(LS_TOKEN);
          setToken('');
          message.warning('令牌已失效，请重新获取');
        } else if (!silent) {
          message.error(`加载文档失败：${(err as Error).message}`);
        }
      } finally {
        if (!silent) setLoading(false);
      }
    },
    [message],
  );

  useEffect(() => {
    void loadDocs(settings, token, statusFilter);
  }, [loadDocs, settings, token, statusFilter]);

  // 有在途任务时自动轮询，直到状态落定
  const hasPending = docs.some((d) => d.status === 'pending' || d.status === 'processing');
  useEffect(() => {
    if (!hasPending || !token) return;
    const timer = window.setTimeout(() => {
      void loadDocs(settings, token, statusFilter, true);
    }, POLL_INTERVAL);
    return () => window.clearTimeout(timer);
  }, [hasPending, docs, token, settings, statusFilter, loadDocs]);

  const handleConnect = async (values: Settings) => {
    setConnecting(true);
    try {
      const res = await issueToken(values);
      localStorage.setItem(LS_SETTINGS, JSON.stringify(values));
      localStorage.setItem(LS_TOKEN, res.token);
      setSettings(values);
      setToken(res.token);
      setSettingsOpen(false);
      message.success(
        `已连接租户 ${values.tenantId}，令牌有效期至 ${new Date(res.expiresAt).toLocaleString()}`,
      );
      void loadDocs(values, res.token, statusFilter);
    } catch (err) {
      message.error(`获取令牌失败：${(err as Error).message}`);
    } finally {
      setConnecting(false);
    }
  };

  const refresh = () => {
    void loadDocs(settings, token, statusFilter);
  };

  return (
    <Layout style={{ minHeight: '100vh', background: '#f6f7f9' }}>
      <Header className="kr-header">
        <Space size={10}>
          <span className="kr-dot" />
          <Title level={5} style={{ margin: 0 }}>
            KnowReply 管理台
          </Title>
        </Space>
        <Space size={12}>
          <Tag color={token ? 'green' : 'default'} style={{ marginInlineEnd: 0 }}>
            {token ? `已连接 · 租户 ${settings.tenantId}` : '未登录'}
          </Tag>
          <Button onClick={() => setSettingsOpen(true)}>连接设置</Button>
        </Space>
      </Header>

      <Content className="kr-content">
        <Tabs
          defaultActiveKey="kb"
          items={[
            {
              key: 'kb',
              label: '知识库',
              children: (
                <Space direction="vertical" size={16} style={{ width: '100%' }}>
                  <UploadPanel settings={settings} token={token} onUploaded={refresh} />
                  <DocTable
                    settings={settings}
                    token={token}
                    docs={docs}
                    loading={loading}
                    statusFilter={statusFilter}
                    onFilterChange={setStatusFilter}
                    onRefresh={refresh}
                    onChanged={refresh}
                  />
                </Space>
              ),
            },
            {
              key: 'search',
              label: '检索测试',
              children: <SearchPanel settings={settings} />,
            },
          ]}
        />
      </Content>

      <SettingsModal
        open={settingsOpen}
        settings={settings}
        loading={connecting}
        onCancel={() => setSettingsOpen(false)}
        onSubmit={handleConnect}
      />
    </Layout>
  );
}
