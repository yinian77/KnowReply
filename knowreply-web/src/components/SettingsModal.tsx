import { useEffect } from 'react';
import { Alert, Form, Input, InputNumber, Modal } from 'antd';
import { DEFAULT_SETTINGS } from '../config';
import { useAuth } from '../context/AuthContext';
import type { Settings } from '../types';

interface Props {
  open: boolean;
  onClose: () => void;
}

export default function SettingsModal({ open, onClose }: Props) {
  const { settings, connecting, connect } = useAuth();
  const [form] = Form.useForm<Settings>();

  useEffect(() => {
    if (open) form.setFieldsValue(settings);
  }, [open, settings, form]);

  const handleOk = async () => {
    const values = await form.validateFields();
    const normalized: Settings = {
      apiBase: values.apiBase.trim(),
      kbBase: values.kbBase.trim(),
      tenantId: Number(values.tenantId) || 1,
      userId: (values.userId ?? '').trim() || 'admin',
    };

    if (await connect(normalized)) onClose();
  };

  return (
    <Modal
      open={open}
      title="连接设置"
      okText="获取访问令牌"
      cancelText="取消"
      confirmLoading={connecting}
      onCancel={onClose}
      onOk={handleOk}
    >
      <Form form={form} layout="vertical" initialValues={DEFAULT_SETTINGS} requiredMark={false}>
        <Form.Item
          name="apiBase"
          label="网关地址"
          rules={[{ required: true, message: '请输入网关地址' }]}
        >
          <Input placeholder="http://127.0.0.1:8080" spellCheck={false} />
        </Form.Item>

        <Form.Item
          name="kbBase"
          label="检索服务地址"
          rules={[{ required: true, message: '请输入检索服务地址' }]}
        >
          <Input placeholder="http://127.0.0.1:8081" spellCheck={false} />
        </Form.Item>

        <Form.Item
          name="tenantId"
          label="租户 ID"
          rules={[{ required: true, message: '请输入租户 ID' }]}
        >
          <InputNumber min={1} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item
          name="userId"
          label="用户标识"
          rules={[{ required: true, message: '请输入用户标识' }]}
        >
          <Input placeholder="admin" spellCheck={false} />
        </Form.Item>
      </Form>

      <Alert
        type="info"
        showIcon
        message="令牌 24 小时有效，仅保存在浏览器 localStorage。当前为联调用的简化登录，正式环境需接账号体系。"
      />
    </Modal>
  );
}
