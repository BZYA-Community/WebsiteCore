<template>
  <n-button v-if="requiresReason" size="small" type="error" :disabled="deleting" @click="open">{{ t('columns.delete') }}</n-button>
  <n-popconfirm v-else @positive-click="remove"><template #trigger><n-button size="small" type="error" :loading="deleting">{{ t('columns.delete') }}</n-button></template>{{ t('columns.confirmRemoval') }}</n-popconfirm>
  <n-modal v-model:show="show" preset="dialog" :title="t('columns.deleteAndReturn')" :positive-text="t('columns.deleteAndReturn')" :negative-text="t('common.cancel')" :loading="deleting" :positive-button-props="{ disabled: !reason.trim() || deleting }" @positive-click="remove">
    <p>{{ t('columns.deleteReturnHint') }}</p>
    <n-input v-model:value="reason" type="textarea" :maxlength="255" show-count :disabled="deleting" :placeholder="t('columns.deleteReasonPlaceholder')" />
  </n-modal>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreUser } from '@/store/user';
import { columnAction, type ColumnItem } from '@/api/columns';
const props = defineProps<{ item: Pick<ColumnItem, 'id' | 'kind' | 'user_id'> }>();
const emit = defineEmits<{ deleted: [] }>();
const { t } = useI18n();
const user = useStoreUser();
const requiresReason = computed(() => props.item.kind === 'article' && props.item.user_id !== user.userInfo.id);
const show = ref(false), reason = ref(''), deleting = ref(false);
function open() {
  reason.value = '';
  show.value = true;
}
async function remove() {
  if (deleting.value || (requiresReason.value && !reason.value.trim())) return false;
  deleting.value = true;
  try {
    await columnAction(props.item.id, 'delete', requiresReason.value ? reason.value.trim() : '');
    show.value = false;
    emit('deleted');
    return true;
  } catch {
    return false;
  } finally {
    deleting.value = false;
  }
}
</script>
