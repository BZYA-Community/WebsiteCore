import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { userInfo as fetchUserInfo } from '@/api/auth';
import { getIdentity } from '@/api/identity';
import { hasPermission as includesPermission, hasAnyPermission as includesAnyPermission } from '@/utils/permissions';

export const TOKEN_KEY = 'PAOPAO_TOKEN';

const emptyUser = () => ({
  id: 0, username: '', nickname: '', created_on: 0,
  follows: 0, followings: 0, tweets_count: 0,
  is_operator: false, permissions: [] as string[],
  identity_groups: [] as Item.IdentityGroup[], identity: '',
});

export const useStoreUser = defineStore('user', () => {
  const userInfo = ref<Partial<Item.UserInfo> & ReturnType<typeof emptyUser>>(emptyUser());
  const guestPermissions = ref<string[]>([]);
  const userLogined = computed(() => userInfo.value.id > 0);
  const subject = computed(() => userLogined.value ? userInfo.value : { permissions: guestPermissions.value });
  let initialized = false;
  let loading: Promise<void> | undefined;
  let sessionRevision = 0;

  const hasPermission = (permission: string) => includesPermission(subject.value, permission);
  const hasAnyPermission = (permissions: readonly string[]) => includesAnyPermission(subject.value, permissions);

  function updateUserinfo(data: Partial<Item.UserInfo>) {
    sessionRevision++;
    loading = undefined;
    userInfo.value = { ...emptyUser(), ...data };
    initialized = true;
  }

  function userLogout() {
    sessionRevision++;
    loading = undefined;
    localStorage.removeItem(TOKEN_KEY);
    userInfo.value = emptyUser();
    initialized = false;
  }

  async function loadSession(force = false): Promise<void> {
    if (loading) return loading;
    if (initialized && !force) return;
    const revision = sessionRevision;
    const token = localStorage.getItem(TOKEN_KEY);
    const pending = (async () => {
      if (token) {
        const current = await fetchUserInfo();
        if (revision !== sessionRevision || localStorage.getItem(TOKEN_KEY) !== token) return;
        userInfo.value = { ...emptyUser(), ...current };
      } else {
        const current = await getIdentity();
        if (revision !== sessionRevision || localStorage.getItem(TOKEN_KEY) !== token) return;
        guestPermissions.value = current.permissions || [];
        userInfo.value = emptyUser();
      }
      initialized = true;
    })().catch(async (error) => {
      const currentToken = localStorage.getItem(TOKEN_KEY);
      if (!token || (revision === sessionRevision && currentToken === token)) throw error;
      if (!currentToken) {
        if (loading === pending) loading = undefined;
        await loadSession();
      }
    });
    loading = pending;
    try {
      await pending;
    } finally {
      if (loading === pending) loading = undefined;
    }
  }

  return { userInfo, userLogined, hasPermission, hasAnyPermission, updateUserinfo, userLogout, loadSession };
});
