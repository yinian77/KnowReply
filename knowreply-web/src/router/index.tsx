import { createBrowserRouter, Navigate } from 'react-router-dom';
import { PATHS } from '../config';
import AdminLayout from '../layouts/AdminLayout';
import KnowledgeBasePage from '../pages/knowledge/KnowledgeBasePage';
import NotFoundPage from '../pages/NotFoundPage';
import RetrievalPage from '../pages/retrieval/RetrievalPage';

/**
 * 路由表：AdminLayout 作为壳层（顶栏 + 导航 + Outlet），页面挂在 children 下。
 * 生产环境由 Nginx 用 try_files 回落到 index.html，否则刷新子路由会 404。
 */
export const router = createBrowserRouter([
  {
    path: PATHS.root,
    element: <AdminLayout />,
    children: [
      { index: true, element: <Navigate to={PATHS.knowledge} replace /> },
      { path: 'knowledge', element: <KnowledgeBasePage /> },
      { path: 'retrieval', element: <RetrievalPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]);
