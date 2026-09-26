import { defineConfig, esmExternalRequirePlugin } from 'vite';
import path from 'path';
import vue from '@vitejs/plugin-vue';
import Components from 'unplugin-vue-components/vite';
import VueI18nPlugin from '@intlify/unplugin-vue-i18n/vite';

import { NaiveUiResolver } from 'unplugin-vue-components/resolvers';
// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: '0.0.0.0',
  },
  plugins: [
    vue({
      template: {
        compilerOptions: {
          // vue-advanced-chat 为 Web Component, 保留原生标签交给运行时
          isCustomElement: (tag) =>
            tag === 'vue-advanced-chat' || tag === 'emoji-picker',
        },
      },
    }),
    Components({
      resolvers: [NaiveUiResolver()],
    }),
    VueI18nPlugin({
      include: [path.resolve(__dirname, './src/locales/**/*.json')],
      // 允许文案中出现 @ | 等字符
      strictMessage: false,
      escapeParameterHtml: false,
    }),
    // esmExternalRequirePlugin({
    //   external: [/^node:/]
    // }),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            return id
              .toString()
              .split('node_modules/')[1]
              .split('/')[0]
              .toString();
          }
        },
      },
    },
  },
});
