/**
 * 认证统一入口（#35）
 *
 * - token 的读 / 写 / 清只有一个出口：当前是 localStorage，
 *   后续若迁移 sessionStorage、内存 + 刷新、或 httpOnly cookie，只改本文件即可。
 * - `Authorization` 请求头只有一个拼接点：request.ts 拦截器、api 模块、
 *   n-upload 组件、axios 直传共用 `authHeader()` / `applyAuthHeader()`。
 * - 附件上传端点与带鉴权的直传请求也收口于此（`attachmentEndpoint()` / `uploadFile()`）。
 *
 * 注意：本文件不依赖 pinia store（只依赖 axios 与环境变量），
 * 以避免 store/user ↔ 本文件的循环引用；
 * 需要 store 的路由级鉴权见 `router/guard.ts`。
 */
import axios, { AxiosRequestConfig, AxiosResponse } from 'axios';

/** 本地存储的用户令牌键名 */
export const TOKEN_KEY = 'PAOPAO_TOKEN';

/** 读取 token（唯一读取点） */
export const getToken = (): string => localStorage.getItem(TOKEN_KEY) || '';

/** 是否持有 token */
export const hasToken = (): boolean => !!getToken();

/** 写入 token（唯一写入点，登录 / 注册成功后调用） */
export const setToken = (token: string): void => {
	localStorage.setItem(TOKEN_KEY, token);
};

/** 清除 token（唯一清除点，登出 / 401 时调用） */
export const clearToken = (): void => {
	localStorage.removeItem(TOKEN_KEY);
};

/** 拼接 `Authorization` 头的值；未登录返回空串 */
export const authHeader = (token: string = getToken()): string =>
	token ? `Bearer ${token}` : '';

/**
 * 往任意 headers 注入 `Authorization`（唯一注入点）。
 *
 * - 兼容 AxiosHeaders（走 `.set()`，保持大小写归一化）与普通对象（n-upload、axios config）；
 * - 未登录时不注入任何头（与原 request.ts 拦截器行为一致）。
 */
export function applyAuthHeader<T extends object = Record<string, unknown>>(
	headers?: T,
	token: string = getToken(),
): T {
	const target = (headers ?? {}) as T & {
		set?: (name: string, value: string) => unknown;
		[key: string]: unknown;
	};
	const value = authHeader(token);
	if (!value) return target;
	if (typeof target.set === 'function') {
		target.set('Authorization', value);
	} else {
		target.Authorization = value;
	}
	return target;
}

/** 附件上传端点（n-upload 的 action 与直传请求共用） */
export const attachmentEndpoint = (): string =>
	import.meta.env.VITE_HOST + '/v1/attachment';

/**
 * 带鉴权的直传请求：替代各视图里手写的 `axios.post(url, form, { headers: { Authorization: ... } })`。
 *
 * 走原生 axios（不经过 request.ts 的响应拦截器），因此响应结构与各调用点原有的
 * `res.data.code` 判断保持一致。
 */
export function uploadFile(
	path: string,
	data: FormData,
	config: AxiosRequestConfig = {},
): Promise<AxiosResponse> {
	return axios.post(import.meta.env.VITE_HOST + path, data, {
		...config,
		headers: applyAuthHeader({ ...(config.headers as Record<string, string>) }),
	});
}
