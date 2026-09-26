import type { MessageApiInjection } from 'naive-ui/lib/message/src/MessageProvider';

type MessageMethod = 'success' | 'warning' | 'error' | 'info';

interface QueuedMessage {
	method: MessageMethod;
	content: string;
}

// 启动期最多缓存的消息条数（超出后丢弃最旧的，只保留最近几条）
const MAX_QUEUE_SIZE = 5;

let api: MessageApiInjection | null = null;
const queue: QueuedMessage[] = [];

const emit = (method: MessageMethod, content: string): void => {
	// 已初始化：直接交给 naive-ui 实例（与 window.$message 同源）
	const target = api ?? window.$message ?? null;
	if (target) {
		target[method](content);
		return;
	}
	// 尚未初始化（首屏请求早于 main-nav/sidebar 挂载）：先排队并打到 console
	console.warn(`[message] (${method}) ${content}`);
	queue.push({ method, content });
	if (queue.length > MAX_QUEUE_SIZE) {
		queue.shift();
	}
};

/**
 * 安全的 $message 封装：初始化前调用不会抛 TypeError，
 * 消息先排队，待 initMessage() 后回放。
 * 用法与 naive-ui 的 useMessage() 一致：message.success/warning/error/info。
 */
export const message = {
	success: (content: string) => emit('success', content),
	warning: (content: string) => emit('warning', content),
	error: (content: string) => emit('error', content),
	info: (content: string) => emit('info', content),
};

/**
 * 初始化消息出口：由 main-nav.vue / sidebar.vue 挂载后调用。
 * 同时写入 window.$message（存量调用点不变），并回放初始化前排队的消息。
 */
export const initMessage = (instance: MessageApiInjection): void => {
	api = instance;
	window.$message = instance;
	const pending = queue.splice(0, queue.length);
	for (const item of pending) {
		instance[item.method](item.content);
	}
};
