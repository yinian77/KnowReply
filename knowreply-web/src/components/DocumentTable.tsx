import { App, Button, Card, Popconfirm, Select, Space, Table, Tag, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useAuth } from '../context/AuthContext';
import { toErrorMessage } from '../services/http';
import { offlineDocument } from '../services/knowledgeApi';
import { STATUS_COLOR, STATUS_TEXT } from '../types';
import type { DocItem } from '../types';

const { Text } = Typography;

interface Props {
  docs: DocItem[];
  loading: boolean;
  statusFilter: string;
  onFilterChange: (value: string) => void;
  onRefresh: () => void;
  onChanged: () => void;
}

const STATUS_OPTIONS = [
  { value: '', label: '全部状态' },
  { value: 'pending', label: 'pending' },
  { value: 'processing', label: 'processing' },
  { value: 'ready', label: 'ready' },
  { value: 'failed', label: 'failed' },
  { value: 'offline', label: 'offline' },
];

export default function DocumentTable({
  docs,
  loading,
  statusFilter,
  onFilterChange,
  onRefresh,
  onChanged,
}: Props) {
  const { settings, token } = useAuth();
  const { message } = App.useApp();

  const handleOffline = async (docId: number) => {
    try {
      await offlineDocument(settings, token, docId);
      message.success(`文档 ${docId} 已下线，切片正在清理`);
      onChanged();
    } catch (err) {
      message.error(`下线失败：${toErrorMessage(err)}`);
    }
  };

  const columns: ColumnsType<DocItem> = [
    { title: 'ID', dataIndex: 'id', width: 64 },
    {
      title: '标题',
      dataIndex: 'title',
      ellipsis: true,
      render: (title: string, row) => (
        <Text title={row.sourceUrl ?? undefined} style={{ fontSize: 13.5 }}>
          {title}
        </Text>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      render: (status: DocItem['status']) => (
        <Tag color={STATUS_COLOR[status]} style={{ marginInlineEnd: 0 }}>
          {STATUS_TEXT[status] ?? status}
        </Tag>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'createdAt',
      width: 180,
      render: (value: string | null) => (
        <Text type="secondary" style={{ fontSize: 12.5 }}>
          {value ? new Date(value).toLocaleString() : '—'}
        </Text>
      ),
    },
    {
      title: '操作',
      width: 90,
      render: (_, row) => (
        <Popconfirm
          title={`确认下线文档 ${row.id}？`}
          description="向量切片将被删除，文档记录保留用于审计。"
          okText="下线"
          cancelText="取消"
          onConfirm={() => handleOffline(row.id)}
          disabled={row.status === 'offline'}
        >
          <Button
            type="link"
            size="small"
            disabled={row.status === 'offline'}
            style={{ padding: 0 }}
          >
            下线
          </Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <Card
      title="文档列表"
      extra={
        <Space>
          <Select
            value={statusFilter}
            options={STATUS_OPTIONS}
            style={{ width: 132 }}
            onChange={onFilterChange}
          />
          <Button onClick={onRefresh}>刷新</Button>
        </Space>
      }
    >
      <Table<DocItem>
        rowKey="id"
        size="middle"
        columns={columns}
        dataSource={docs}
        loading={loading}
        pagination={false}
        scroll={{ x: 720 }}
        locale={{ emptyText: token ? '暂无文档，先上传一份试试' : '请先在连接设置中获取访问令牌' }}
      />
    </Card>
  );
}
