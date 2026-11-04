import { useState } from 'react';
import { App, Button, Card, Input, Space, Typography, Upload } from 'antd';
import { InboxOutlined } from '@ant-design/icons';
import type { UploadFile, UploadProps } from 'antd';
import { uploadDocument } from '../api';
import type { Settings } from '../types';

const { Dragger } = Upload;
const { Text } = Typography;

const ACCEPT = '.md,.markdown,.txt,.docx,.pdf';

interface Props {
  settings: Settings;
  token: string;
  onUploaded: () => void;
}

export default function UploadPanel({ settings, token, onUploaded }: Props) {
  const { message } = App.useApp();
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [title, setTitle] = useState('');
  const [uploading, setUploading] = useState(false);

  const draggerProps: UploadProps = {
    name: 'file',
    multiple: false,
    maxCount: 1,
    accept: ACCEPT,
    fileList,
    // 关闭自动上传，交由按钮统一提交，便于同时带上标题
    beforeUpload: () => false,
    onChange: ({ fileList: next }) => setFileList(next.slice(-1)),
    onRemove: () => setFileList([]),
  };

  const handleUpload = async () => {
    const file = fileList[0]?.originFileObj;
    if (!file) {
      message.warning('请先选择文件');
      return;
    }
    if (!token) {
      message.warning('请先在连接设置中获取访问令牌');
      return;
    }

    setUploading(true);
    try {
      const res = await uploadDocument(settings, token, file as File, title.trim() || undefined);
      message.success(`已入队：docId=${res.docId}，任务 ${res.taskId}`);
      setFileList([]);
      setTitle('');
      onUploaded();
    } catch (err) {
      message.error(`上传失败：${(err as Error).message}`);
    } finally {
      setUploading(false);
    }
  };

  return (
    <Card title="上传文档" variant="outlined">
      <Dragger {...draggerProps} style={{ marginBottom: 16 }}>
        <p className="ant-upload-drag-icon">
          <InboxOutlined />
        </p>
        <p className="ant-upload-text">点击或拖拽文件到此区域</p>
        <p className="ant-upload-hint">支持 MD / TXT / DOCX / PDF，单文件 ≤ 50MB</p>
      </Dragger>

      <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
        <Input
          placeholder="标题（留空则取文件名）"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          allowClear
        />
        <Button type="primary" loading={uploading} onClick={handleUpload}>
          上传并入库
        </Button>
      </Space.Compact>

      <Text type="secondary" style={{ fontSize: 12.5 }}>
        上传后进入 asynq 队列，由 worker 异步解析、切分、向量化；状态会自动刷新。
      </Text>
    </Card>
  );
}
