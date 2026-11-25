import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
export default defineConfig({
  plugins: [vue()],
  build: {
    sourcemap: false,
    rollupOptions: {
      output: {
        // 页面按实际入口惰性加载，公式与Markdown依赖再各自成包；保留默认体积告警门槛。
        manualChunks(id) {
          const path = id.replaceAll('\\', '/');
          if (path.includes('/node_modules/katex/')) return 'formula';
          if (path.includes('/node_modules/markdown-it/')) return 'markdown';
        },
      },
    },
  },
});
