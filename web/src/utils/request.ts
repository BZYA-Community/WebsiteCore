import axios, { AxiosRequestConfig } from 'axios';
import { useStoreMain } from '@/store/main';
import { applyAuthHeader, clearToken } from '@/composables/useAuth';

const service = axios.create({
	baseURL: import.meta.env.VITE_HOST,
	timeout: 30000,
});

service.interceptors.request.use(
	(config) => {
		// 鉴权Header（统一由 useAuth 注入，未登录时不注入）
		applyAuthHeader(config.headers);

		return config;
	},
	(error) => {
		return Promise.reject(error);
	},
);

service.interceptors.response.use(
	(response) => {
		const { data = {}, code = 0 } = response?.data || {};
		if (+code === 0) {
			return data || {};
		} else {
			Promise.reject(response?.data || {});
		}
	},
	(error = {}) => {
		const { response = {} } = error || {};
		// 重定向
		if (+response?.status === 401) {
			clearToken();

			if (response?.data.code !== 10005) {
				window.$message.warning(response?.data.msg || '鉴权失败');
			} else {
				// 打开登录弹窗
				useStoreMain().triggerAuth(true);
			}
		} else {
			window.$message.error(response?.data?.msg || '请求失败');
		}
		return Promise.reject(response?.data || {});
	},
);

export default service;

export function request<T, R>(config: AxiosRequestConfig<T>): Promise<R> {
	return service(config) as unknown as Promise<R>;
}

/**
 * 创建一个API对象，支持链式调用
 */
export function createApi<T>(): Readonly<T> {
	const createProxy = (...names: string[]) => {
		const exec = (...args: unknown[]) => {
			const _path: string[] = [];
			let method: 'get' | 'post' | '' = '';
			const methods = ['get', 'post'];

			for (const name of names) {
				const lowerName = name.toLowerCase();
				if (methods.includes(lowerName) && !method) {
					method = lowerName as 'get' | 'post';
				} else {
					_path.push(name);
				}
			}
			if (!method) method = 'get';

			// 如果最后一条路径是 _self 则代表不需要它，直接去掉它
			if (_path[_path.length - 1] === "_self") _path.pop();

			return request({
				method,
				url: _path.join('/'),
				...(method === 'get' ? { params: args[0] } : { data: args[0] }),
			});
		};
		return new Proxy(exec, {
			get(target, p: string) {
				if (p === 'then') return undefined;
				const t = target as unknown as Record<string, unknown>;
				if (!t[p]) t[p] = createProxy(...names, p);
				return t[p];
			},
		});
	};
	return createProxy() as unknown as Readonly<T>;
}

export const Api = createApi<Api>();