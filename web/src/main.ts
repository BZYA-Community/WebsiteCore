import { createApp } from 'vue';
import { createPinia } from 'pinia';
import router from './router';
import App from './App.vue';
import { register as registerChat } from 'vue-advanced-chat';
import '@/assets/css/main.less';

import type { MessageApiInjection } from 'naive-ui/lib/message/src/MessageProvider';

// 通用字体
import 'vfonts/Lato.css';
// 等宽字体
import 'vfonts/FiraCode.css';

// 注册私信聊天 Web Component (vue-advanced-chat)
registerChat();

const pinia = createPinia();

// pinia 必须先于 router 安装：路由守卫（router/guard.ts）会在组件外读取 store
createApp(App).use(pinia).use(router).mount('#app');

declare global {
  interface Window {
    $message: MessageApiInjection;
  }
}
