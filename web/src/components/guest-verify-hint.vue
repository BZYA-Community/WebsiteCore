<template>
    <div class="compose-wrap">
        <n-alert type="info">{{ t('identity.guestHint') }}</n-alert>
        <n-alert v-if="!profile.contactVerificationAvailable" type="warning" style="margin-top: 8px">
            {{ t('identity.guestVerifyUnavailable') }}
        </n-alert>
        <div class="login-wrap">
            <n-button
                v-if="profile.contactVerificationAvailable"
                strong
                secondary
                round
                type="primary"
                @click="router.push({ name: 'setting' })"
            >
                {{ t('identity.guestVerifyAction') }}
            </n-button>
        </div>
    </div>
</template>

<script setup lang="ts">
import { useStoreProfile } from '@/store/profile';
import { useRouter } from 'vue-router';
import { storeToRefs } from 'pinia';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const router = useRouter();
const { profile } = storeToRefs(useStoreProfile());
</script>
