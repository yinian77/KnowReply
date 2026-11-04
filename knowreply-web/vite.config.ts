import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// 开发期直连网关（6701）与检索服务（6702），后端已对 dev 环境放开 CORS。
// 生产环境由 Nginx 同源反代，不需要下面的代理配置。
export default defineConfig({
  plugins: [react()],
  server: {
    host: '127.0.0.1',
    port: 6703,
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    rollupOptions: {
      output: {
        // antd 体积大，单独切出来让浏览器缓存更稳定
        manualChunks: {
          react: ['react', 'react-dom'],
          antd: ['antd', '@ant-design/icons'],
        },
      },
    },
  },
});
