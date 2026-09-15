import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: '/dashboard/',
  build: { outDir: '../bap-controlplane/internal/api/web/dashboard-build', emptyOutDir: true },
  server: {
    proxy: {
      '/api': { target: process.env.BAP_DEV_SERVER || 'http://localhost:8080', changeOrigin: false },
      '/assets/admin-client.js': { target: process.env.BAP_DEV_SERVER || 'http://localhost:8080', changeOrigin: false },
      '/inspector': { target: process.env.BAP_DEV_SERVER || 'http://localhost:8080', changeOrigin: false },
    },
  },
});
