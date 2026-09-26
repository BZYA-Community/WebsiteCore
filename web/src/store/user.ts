import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { clearToken } from "@/composables/useAuth";

/** 本地存储的用户令牌键名（读写统一收口于 @/composables/useAuth，此处保留导出兼容旧引用） */
export { TOKEN_KEY } from "@/composables/useAuth";

export const useStoreUser = defineStore('user', () => {
    const userInfo = ref<Record<string, any>>({
        id: 0,
        username: '',
        nickname: '',
        created_on: 0,
        follows: 0,
        followings: 0,
        tweets_count: 0,
        is_admin: false,
        roles: [] as string[],
        identity: '',
    });

    const userLogined = computed(() => userInfo.value.id > 0);

    /** 判断当前用户是否持有指定管理角色 */
    const hasRole = (role: string) => (userInfo.value.roles || []).includes(role);

    function updateUserinfo(data: Record<string, any>) {
        userInfo.value = data;
    }

    function userLogout() {
        clearToken();
        userInfo.value = {
            id: 0,
            nickname: '',
            username: '',
            created_on: 0,
            follows: 0,
            followings: 0,
            tweets_count: 0,
            is_admin: false,
            roles: [],
            identity: '',
        };
    }

    return {
        userInfo,
        userLogined,
        hasRole,
        updateUserinfo, userLogout,
    }
});