<template>
    <n-modal
        :show="!!storeUser.userInfo.must_change_password"
        :mask-closable="false"
        :close-on-esc="false"
        :closable="false"
        preset="card"
        :title="t('auth.mustChangePassword')"
        style="width: min(440px, 94vw)"
    >
        <n-form @submit.prevent="save">
            <n-form-item :label="t('setting.password.oldLabel')">
                <n-input v-model:value="oldPassword" type="password" autocomplete="current-password" />
            </n-form-item>
            <n-form-item :label="t('setting.password.newLabel')">
                <n-input v-model:value="password" type="password" autocomplete="new-password" />
            </n-form-item>
            <n-form-item :label="t('setting.password.repeatLabel')">
                <n-input v-model:value="repeatPassword" type="password" autocomplete="new-password" />
            </n-form-item>
            <n-space justify="end">
                <n-button @click="storeUser.userLogout()">{{ t('common.cancel') }}</n-button>
                <n-button
                    type="primary"
                    attr-type="submit"
                    :loading="saving"
                    :disabled="!oldPassword || !password || password === oldPassword || password !== repeatPassword"
                >
                    {{ t('setting.password.update') }}
                </n-button>
            </n-space>
        </n-form>
    </n-modal>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Api } from '@/utils/request';
import { useStoreUser } from '@/store/user';
import { useStoreMain } from '@/store/main';

const { t } = useI18n();
const storeUser = useStoreUser();
const oldPassword = ref('');
const password = ref('');
const repeatPassword = ref('');
const saving = ref(false);
watch(() => storeUser.userInfo.id, () => {
    oldPassword.value = password.value = repeatPassword.value = '';
});
const save = async () => {
    if (password.value !== repeatPassword.value || saving.value) return;
    saving.value = true;
    try {
        await Api.v1.user.post.password({ old_password: oldPassword.value, password: password.value });
        oldPassword.value = password.value = repeatPassword.value = '';
        storeUser.userLogout();
        useStoreMain().triggerAuth(true);
        window.$message.success(t('auth.passwordChanged'));
    } catch (_err) {
        // Errors are displayed by the request interceptor.
    } finally { saving.value = false; }
};
</script>
