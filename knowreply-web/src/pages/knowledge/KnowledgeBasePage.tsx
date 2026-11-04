import { useState } from 'react';
import { Space } from 'antd';
import DocumentTable from '../../components/DocumentTable';
import UploadPanel from '../../components/UploadPanel';
import { useDocumentList } from '../../hooks/useDocumentList';

export default function KnowledgeBasePage() {
  const [statusFilter, setStatusFilter] = useState('');
  const { docs, loading, reload } = useDocumentList(statusFilter);

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <UploadPanel onUploaded={reload} />
      <DocumentTable
        docs={docs}
        loading={loading}
        statusFilter={statusFilter}
        onFilterChange={setStatusFilter}
        onRefresh={reload}
        onChanged={reload}
      />
    </Space>
  );
}
