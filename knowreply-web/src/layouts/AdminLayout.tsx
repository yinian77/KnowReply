import { useState } from 'react';
import { Layout, Menu, Space, Tag, Typography, Button } from 'antd';
import { Link, Outlet, useLocation } from 'react-router-dom';
import SettingsModal from '../components/SettingsModal';
import { PATHS } from '../config';
import { useAuth } from '../context/AuthContext';

const { Header, Content } = Layout;
const { Title } = Typography;

const MENU_ITEMS = [
  { key: PATHS.knowledge, label: <Link to={PATHS.knowledge}>知识库</Link> },
  { key: PATHS.retrieval, label: <Link to={PATHS.retrieval}>检索测试</Link> },
];

export default function AdminLayout() {
  const { connected, settings } = useAuth();
  const location = useLocation();
  const [settingsOpen, setSettingsOpen] = useState(false);

  return (
    <Layout style={{ minHeight: '100vh', background: '#f6f7f9' }}>
      <Header className="kr-header">
        <Space size={28}>
          <Space size={10}>
            <span className="kr-dot" />
            <Title level={5} style={{ margin: 0 }}>
              KnowReply
            </Title>
          </Space>
          <Menu
            mode="horizontal"
            className="kr-menu"
            selectedKeys={[location.pathname]}
            items={MENU_ITEMS}
          />
        </Space>

        <Space size={12}>
          <Tag color={connected ? 'green' : 'default'} style={{ marginInlineEnd: 0 }}>
            {connected ? `已连接 · 租户 ${settings.tenantId}` : '未登录'}
          </Tag>
          <Button onClick={() => setSettingsOpen(true)}>连接设置</Button>
        </Space>
      </Header>

      <Content className="kr-content">
        <Outlet />
      </Content>

      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
    </Layout>
  );
}
