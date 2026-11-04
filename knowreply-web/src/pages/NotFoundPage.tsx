import { Button, Result } from 'antd';
import { Link } from 'react-router-dom';
import { PATHS } from '../config';

export default function NotFoundPage() {
  return (
    <Result
      status="404"
      title="404"
      subTitle="页面不存在，可能是地址输错了。"
      extra={
        <Link to={PATHS.knowledge}>
          <Button type="primary">返回知识库</Button>
        </Link>
      }
    />
  );
}
