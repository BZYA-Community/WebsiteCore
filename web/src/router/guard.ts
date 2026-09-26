import type { RouteLocationRaw } from 'vue-router';
import { useStoreMain } from '@/store/main';
import { useStoreUser } from '@/store/user';
import { hasToken } from '@/composables/useAuth';
import { userInfo as fetchUserInfo } from '@/api/auth';

declare module 'vue-router' {
	interface RouteMeta {
		/** 需要登录 */
		requiresAuth?: boolean;
		/** 需要管理员（is_admin） */
		requiresAdmin?: boolean;
		/** 除管理员外额外放行的角色（如审核员） */
		adminRoles?: string[];
	}
}

/** 路由 meta 中参与鉴权的字段 */
export interface RouteAuthMeta {
	requiresAuth?: boolean;
	requiresAdmin?: boolean;
	adminRoles?: string[];
}

/**
 * 路由级鉴权（#35）：集中替代 AdminUsers / AdminSettings / AdminAudit 里的
 * `ensureAdminAccess()` / `ensureAuditAccess()` 与 ComposeMd 的 `ensureLogin()`
 * 三份复制实现。
 *
 * 返回 `null` 表示放行，否则返回重定向目标（由 router.beforeEach 执行）：
 * - 未登录（无 token）：打开登录弹窗并回广场；
 * - 有 token 但用户信息未加载（刷新 / 直达）：先拉取一次，失败则登出回广场；
 * - 非管理员访问管理路由：按 404 处理（不暴露入口）。
 */
export async function checkRouteAuth(
	meta: RouteAuthMeta,
): Promise<RouteLocationRaw | null> {
	const storeMain = useStoreMain();
	const storeUser = useStoreUser();

	// 1. 无 token：未登录（或已被 401 清除）——弹登录框并回广场
	if (!hasToken()) {
		if (storeUser.userInfo.id > 0) {
			// token 已失效但 store 里还残留旧用户信息，一并清掉
			storeUser.userLogout();
		}
		storeMain.triggerAuth(true);
		storeMain.triggerAuthKey('signin');
		return { name: 'home' };
	}

	// 2. 有 token 但用户信息未加载：拉取一次（替代组件里各自的 fetchUserInfo）
	if (storeUser.userInfo.id === 0) {
		try {
			const currentUser = await fetchUserInfo();
			storeUser.updateUserinfo(currentUser);
		} catch {
			storeUser.userLogout();
			return { name: 'home' };
		}
	}

	// 3. 管理员校验：is_admin 或 meta.adminRoles 中的任一角色，不满足按 404 处理
	if (meta.requiresAdmin) {
		const allowed =
			!!storeUser.userInfo.is_admin ||
			(meta.adminRoles || []).some((role) => storeUser.hasRole(role));
		if (!allowed) return { name: '404' };
	}

	return null;
}
