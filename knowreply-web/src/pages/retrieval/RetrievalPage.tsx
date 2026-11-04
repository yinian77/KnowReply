import { useState } from 'react';
import {
  App,
  Button,
  Card,
  Empty,
  Input,
  InputNumber,
  Space,
  Spin,
  Tag,
  Typography,
} from 'antd';
import { useAuth } from '../../context/AuthContext';
import { toErrorMessage } from '../../services/http';
import { searchChunks } from '../../services/retrievalApi';
import type { Hit, SearchResponse } from '../../types';

const { Text, Paragraph } = Typography;

function ranks(hit: Hit): string[] {
  const out: string[] = [];
  if (hit.vectorRank) out.push(`向量#${hit.vectorRank}`);
  if (hit.textRank) out.push(`全文#${hit.textRank}`);
  if (hit.gramRank) out.push(`二元组#${hit.gramRank}`);
  return out;
}

export default function RetrievalPage() {
  const { settings } = useAuth();
  const { message } = App.useApp();

  const [query, setQuery] = useState('');
  const [limit, setLimit] = useState(5);
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<SearchResponse | null>(null);

  const handleSearch = async () => {
    const q = query.trim();
    if (!q) {
      message.warning('请输入查询内容');
      return;
    }

    setLoading(true);
    try {
      setResult(await searchChunks(settings, q, limit));
    } catch (err) {
      message.error(`检索失败：${toErrorMessage(err)}`);
      setResult(null);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card title="检索测试">
      <Space.Compact style={{ width: '100%', marginBottom: 8 }}>
        <Input
          placeholder="输入真实用户问法，例如：退款几天到账"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onPressEnter={handleSearch}
          allowClear
        />
        {/* antd 5.24 起 InputNumber 的 addonBefore 已废弃，改用 placeholder 提示（外层已是 Space.Compact） */}
        <InputNumber
          min={1}
          max={20}
          value={limit}
          onChange={(v) => setLimit(Number(v) || 5)}
          placeholder="条数"
          aria-label="返回条数"
          style={{ width: 96 }}
        />
        <Button type="primary" loading={loading} onClick={handleSearch}>
          检索
        </Button>
      </Space.Compact>

      <Text type="secondary" style={{ fontSize: 12.5 }}>
        三路召回：向量 + 全文 + 中文二元组，RRF 融合排序。用于判断问题出在切分还是模型。
      </Text>

      <Spin spinning={loading} style={{ display: 'block', marginTop: 16 }}>
        {result && (
          <>
            <Space size={16} wrap style={{ marginBottom: 12 }}>
              <Text type="secondary">向量召回 {result.vectorHits}</Text>
              <Text type="secondary">全文召回 {result.textHits}</Text>
              <Text type="secondary">二元组召回 {result.gramHits}</Text>
              <Text type="secondary">耗时 {result.tookMs}ms</Text>
              <Tag color={result.embedder.startsWith('mock') ? 'orange' : 'green'}>
                {result.embedder}
              </Tag>
            </Space>

            {result.hits.length === 0 ? (
              <Empty description="没有命中任何切片" />
            ) : (
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                {result.hits.map((hit) => (
                  <Card key={hit.chunkId} size="small" styles={{ body: { padding: 14 } }}>
                    <Space
                      style={{ width: '100%', justifyContent: 'space-between', marginBottom: 6 }}
                      wrap
                    >
                      <Text strong>{hit.headingPath || '（无标题路径）'}</Text>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        doc {hit.docId} · {ranks(hit).join(' / ') || '—'} · fused{' '}
                        {hit.fusedScore.toFixed(4)}
                      </Text>
                    </Space>
                    <Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap', fontSize: 13 }}>
                      {hit.content}
                    </Paragraph>
                  </Card>
                ))}
              </Space>
            )}
          </>
        )}
      </Spin>
    </Card>
  );
}
