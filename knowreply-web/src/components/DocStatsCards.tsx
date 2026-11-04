import { Card, Col, Row, Statistic, Tooltip } from 'antd';
import type { DocStats } from '../types';
import { formatBytes } from '../utils/format';

interface Props {
  stats: DocStats | null;
}

export default function DocStatsCards({ stats }: Props) {
  const s: DocStats = stats ?? {
    total: 0,
    pending: 0,
    processing: 0,
    ready: 0,
    failed: 0,
    offline: 0,
    chunks: 0,
    bytes: 0,
  };

  const items = [
    { title: '文档总数', value: s.total, hint: '当前租户已上传的文档数' },
    { title: '已入库', value: s.ready, valueStyle: { color: '#1a7f4b' }, hint: '切片已写入向量库，可被检索' },
    { title: '处理中', value: s.pending + s.processing, valueStyle: { color: '#2459c8' }, hint: 'pending + processing，队列在处理' },
    { title: '失败', value: s.failed, valueStyle: { color: '#c0392b' }, hint: '解析或向量化失败，可在列表查看原因' },
    { title: '切片总数', value: s.chunks, hint: 'kb_chunks 中可检索的片段数量' },
    { title: '存储占用', value: formatBytes(s.bytes), hint: '源文件总体积' },
  ];

  return (
    <Row gutter={[12, 12]}>
      {items.map((item) => (
        <Col key={item.title} xs={12} sm={8} lg={4}>
          <Tooltip title={item.hint}>
            <Card size="small" styles={{ body: { padding: '12px 16px' } }}>
              <Statistic
                title={item.title}
                value={item.value}
                valueStyle={{ fontSize: 20, ...item.valueStyle }}
              />
            </Card>
          </Tooltip>
        </Col>
      ))}
    </Row>
  );
}
