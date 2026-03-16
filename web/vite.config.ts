import react from '@vitejs/plugin-react';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const backendTarget = env.VITE_BACKEND_TARGET || 'http://localhost:4569';
  const appBasePath = env.VITE_APP_BASE_PATH || (mode === 'production' ? './' : '/');

  return {
    base: appBasePath,
    plugins: [react()],
    server: {
      port: 4572,
      strictPort: true,
      host: '0.0.0.0',
      proxy: {
        '/api': {
          target: backendTarget,
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api/, ''),
        },
      },
    },
  };
});
