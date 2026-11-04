import { App, Button, Card, Popconfirm, Select, Space, Table, Tag, Tooltip, Typography } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useAuth } from '../context/AuthContext';
import { toErrorMessage } from '../services/http';
import { offlineDocument } from '../services/knowledgeApi';
import { STATUS_COLOR, STATUS_TEXT } from '../types';
import type { DocItem } from '../types';
import { formatBytes, formatRelative } from '../utils/format';

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

/** 从 source_url 里取文件后缀，作为“文档类型” */
function fileType(url: string | null): string {
  if (!url) return '—';
  const match = /\.([a-z0-9]+)(?:\?|#|$)/i.exec(url);
  return match ? match[1].toUpperCase() : '—';
}

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
    { title: 'ID', dataIndex: 'id', width: 60 },
    {
      title: '文档',
      dataIndex: 'title',
      ellipsis: true,
      render: (title: string, row) => (
        <Space direction="vertical" size={0}>
          <Text style={{ fontSize: 13.5 }}>{title}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {fileType(row.sourceUrl)} · v{row.version}
            {row.effectiveAt ? ` · 生效 ${row.effectiveAt}` : ''}
          </Text>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 104,
      render: (status: DocItem['status'], row) => {
        const tag = (
          <Tag color={STATUS_COLOR[status]} style={{ marginInlineEnd: 0 }}>
            {STATUS_TEXT[status] ?? status}
          </Tag>
        );
        // 失败时把原因挂在悬浮提示上，运营不用点进详情
        return row.errorMsg ? (
          <Tooltip title={row.errorMsg} overlayStyle={{ maxWidth: 360 }}>
            {tag}
          </Tooltip>
        ) : (
          tag
        );
      },
    },
    {
      title: '切片数',
      dataIndex: 'chunkCount',
      width: 84,
      align: 'right',
      render: (count: number, row) =>
        row.status === 'ready' ? (
          <Text>{count}</Text>
        ) : (
          <Text type="secondary">—</Text>
        ),
    },
    {
      title: '大小',
      dataIndex: 'sourceSize',
      width: 92,
      align: 'right',
      render: (size: number) => (
        <Text type="secondary" style={{ fontSize: 12.5 }}>
          {formatBytes(size)}
        </Text>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      width: 132,
      render: (updatedAt: string | null, row) => {
        const iso = updatedAt ?? row.createdAt;
        return (
          <Tooltip title={iso ? new Date(iso).toLocaleString() : '—'}>
            <Text type="secondary" style={{ fontSize: 12.5 }}>
              {formatRelative(iso)}
            </Text>
          </Tooltip>
        );
      },
    },
    {
      title: '操作',
      width: 80,
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
      title="已录入文档"
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
        scroll={{ x: 760 }}
        locale={{ emptyText: token ? '暂无文档，先上传一份试试' : '请先在连接设置中获取访问令牌' }}
      />
    </Card>
  );
}
