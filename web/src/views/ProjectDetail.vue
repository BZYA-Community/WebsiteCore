<template>
  <div>
    <main-nav :title="t('project.detailPageTitle')" :back="true" />
    <main class="project-detail main-content-wrap">
      <n-spin :show="loading">
        <n-result v-if="notFound" status="404" :title="t('project.notFound')" />
        <n-result v-else-if="loadFailed" status="error" :title="t('project.loadFailed')">
          <template #footer><n-button @click="loadProject">{{ t('project.retry') }}</n-button></template>
        </n-result>
        <template v-else-if="project">
          <section class="detail-header">
            <div class="detail-heading">
              <div class="creator">
                <n-avatar round :size="34" :src="project.creator.avatar" />
                <router-link :to="{ name: 'user', query: { s: project.creator.username } }">{{ project.creator.nickname }}</router-link>
              </div>
              <h1>{{ project.title }}</h1>
              <div class="detail-tags">
                <n-tag v-for="tag in project.tags" :key="tag" round type="success">{{ tag }}</n-tag>
              </div>
            </div>
            <div class="project-actions">
              <n-button v-if="canEdit" :disabled="following" secondary @click="openEditor">
                <template #icon><n-icon><create-outline /></n-icon></template>
                {{ t('project.edit') }}
              </n-button>
              <n-button :type="project.followed ? 'default' : 'primary'" :loading="following" @click="toggleFollow">
                <template #icon><n-icon><heart v-if="project.followed" /><heart-outline v-else /></n-icon></template>
                {{ project.followed ? t('project.following') : t('project.follow') }} · {{ project.follow_count }}
              </n-button>
              <n-button secondary disabled>
                <template #icon><n-icon><people-outline /></n-icon></template>
                {{ t('project.joinComingSoon') }}
              </n-button>
            </div>
          </section>

          <section class="detail-links" :aria-label="t('project.projectLinks')">
            <n-button type="primary" tag="a" :href="project.url" target="_blank" rel="noopener noreferrer">
              <template #icon><n-icon><open-outline /></n-icon></template>
              {{ t('project.visitProject') }}
            </n-button>
            <n-button v-if="project.post_id" @click="openDiscussion">
              <template #icon><n-icon><chatbox-outline /></n-icon></template>
              {{ t('project.viewDiscussion') }}
            </n-button>
          </section>

          <section class="detail-gallery" :class="{ single: project.images.length === 1 }">
            <n-image v-for="(image, index) in project.images" :key="image + index" :src="image" :alt="project.title" object-fit="cover" lazy />
          </section>

          <div class="detail-columns">
            <article class="project-readme">
              <span class="section-kicker">{{ t('project.detailKicker') }}</span>
              <md-preview :model-value="prepareMdRender(project.intro)" :theme="editorTheme" no-mermaid no-katex no-html />
            </article>
            <aside>
              <section class="side-card">
                <h2>{{ t('project.members') }}</h2>
                <div v-for="member in project.members" :key="member.user.id" class="member-row">
                  <n-avatar round :size="30" :src="member.user.avatar" />
                  <span>{{ member.user.nickname }}</span>
                  <n-tag size="small" :bordered="false">{{ member.role === 'owner' ? t('project.owner') : t('project.member') }}</n-tag>
                </div>
              </section>
            </aside>
          </div>
        </template>
      </n-spin>
    </main>

    <n-modal v-model:show="showEditor" preset="card" :title="t('project.editTitle')" class="project-editor-modal" :mask-closable="!submitting" :closable="!submitting" :close-on-esc="!submitting">
      <n-form label-placement="top" @submit.prevent="submitEdit">
        <n-form-item :label="t('project.titleLabel')" required>
          <n-input v-model:value="editDraft.title" :maxlength="30" show-count :placeholder="t('project.titlePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('project.introLabel')" required>
          <md-editor
            v-model="editDraft.intro"
            class="project-markdown-editor"
            :theme="editorTheme"
            :max-length="800"
            :placeholder="t('project.introPlaceholder')"
            :toolbars-exclude="editToolbarsExclude"
            no-mermaid
            no-katex
            no-html
          />
        </n-form-item>
        <n-form-item :label="t('project.tagsLabel')">
          <n-select
            :value="editDraft.tags"
            :disabled="submitting"
            @update:value="updateTags"
            multiple
            tag
            filterable
            :options="presetTagOptions"
            :max-tag-count="8"
            :placeholder="t('project.tagsPlaceholder')"
            class="tags-select"
          />
        </n-form-item>
        <n-form-item :label="t('project.imagesLabel')" required>
          <div class="upload-field">
            <n-upload
              ref="uploadRef"
              v-model:file-list="fileQueue"
              list-type="image-card"
              accept="image/webp,image/png,image/jpeg,image/gif"
              :action="uploadGateway"
              :headers="{ Authorization: uploadToken }"
              :data="{ type: 'public/image' }"
              :max="6"
              :disabled="submitting"
              @before-upload="beforeUpload"
              @finish="finishUpload"
              @error="failUpload"
              @remove="removeUpload"
            >
              {{ t('project.uploadImage') }}
            </n-upload>
            <div class="url-input-row">
              <n-input
                v-model:value="imageUrlInput"
                :disabled="submitting"
                type="url"
                :placeholder="t('project.imageUrlPlaceholder')"
                class="url-input"
                @keydown.enter.prevent="addImageUrl"
              />
              <n-button size="small" :disabled="submitting || !imageUrlInput.trim()" @click="addImageUrl">
                {{ t('project.addImageUrl') }}
              </n-button>
            </div>
            <small>{{ t('project.imagesHelp') }}</small>
          </div>
        </n-form-item>
        <n-form-item :label="t('project.linkLabel')" required :validation-status="linkError ? 'error' : undefined" :feedback="linkError">
          <n-input v-model:value="editDraft.url" type="url" placeholder="https://" @blur="validateLink">
            <template #prefix><n-icon><open-outline /></n-icon></template>
          </n-input>
        </n-form-item>
        <div class="editor-actions">
          <n-button :disabled="submitting" @click="showEditor = false">{{ t('common.cancel') }}</n-button>
          <n-button attr-type="submit" type="primary" :loading="submitting" :disabled="uploading">{{ t('project.saveChanges') }}</n-button>
        </div>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { followProject, getProject, unfollowProject, updateProject } from '@/api/project';
import type { ProjectItem } from '@/api/project';
import { useMediaUpload } from '@/composables/useMediaUpload';
import { useStoreMain } from '@/store/main';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { mdTheme, prepareMdRender } from '@/utils/markdown';
import { ChatboxOutline, CreateOutline, Heart, HeartOutline, OpenOutline, PeopleOutline } from '@vicons/ionicons5';
import { MdEditor, MdPreview } from 'md-editor-v3';
import type { ToolbarNames } from 'md-editor-v3';
import 'md-editor-v3/lib/style.css';
import type { UploadFileInfo, UploadInst, UploadOnFinish } from 'naive-ui';
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const storeMain = useStoreMain();
const storeUser = useStoreUser();
const editorTheme = computed(() => mdTheme(storeMain.theme));
const project = ref<ProjectItem>();
const loading = ref(false);
const following = ref(false);
const loadFailed = ref(false);
const notFound = ref(false);
const projectID = computed(() => Number(route.query.id));
let loadRequestRevision = 0;
let followRequestRevision = 0;

function hasErrorCode(error: unknown, code: number) {
  return typeof error === 'object' && error !== null
    && Number((error as { code?: unknown }).code) === code;
}

async function loadProject() {
  const revision = ++loadRequestRevision;
  showEditor.value = false;
  const id = projectID.value;
  ++followRequestRevision;
  project.value = undefined;
  following.value = false;
  loadFailed.value = false;
  notFound.value = false;
  if (!Number.isInteger(id) || id <= 0) {
    notFound.value = true;
    loading.value = false;
    return;
  }
  loading.value = true;
  try {
    const result = await getProject(id);
    if (revision !== loadRequestRevision) return;
    project.value = result.project;
  } catch (error) {
    if (revision !== loadRequestRevision) return;
    if (hasErrorCode(error, 30021)) notFound.value = true;
    else loadFailed.value = true;
  } finally {
    if (revision === loadRequestRevision) loading.value = false;
  }
}

async function toggleFollow() {
  if (!project.value || following.value || showEditor.value) return;
  if (!storeUser.userLogined) {
    storeMain.triggerAuth(true);
    storeMain.triggerAuthKey('signin');
    return;
  }
  if (!storeUser.hasPermission('community.interact')) {
    window.$message.warning(t('project.followNoPermission'));
    return;
  }
  const id = project.value.id;
  const followed = project.value.followed;
  const revision = ++followRequestRevision;
  following.value = true;
  try {
    const result = followed ? await unfollowProject(id) : await followProject(id);
    if (revision !== followRequestRevision || project.value?.id !== id) return;
    project.value.followed = result.status;
    project.value.follow_count = result.follow_count;
  } finally {
    if (revision === followRequestRevision) following.value = false;
  }
}

function openDiscussion() {
  if (project.value?.post_id) void router.push({ name: 'post', query: { id: project.value.post_id } });
}

// --- Edit ---
const isOwner = computed(() =>
  project.value && storeUser.userLogined && storeUser.userInfo.id
    ? project.value.creator_id === storeUser.userInfo.id
    : false,
);
const canEdit = computed(() => isOwner.value && storeUser.hasPermission('post.create') && storeUser.hasPermission('content.upload'));
const showEditor = ref(false);
const submitting = ref(false);
const linkError = ref('');
const editDraft = reactive({ title: '', intro: '', url: '', tags: [] as string[] });

// Upload state for edit dialog
const uploadRef = ref<UploadInst>();
const fileQueue = ref<UploadFileInfo[]>([]);
const uploadedImages = ref<{ id: string; content: string }[]>([]);
const imageUrlInput = ref('');
let nextImageID = 0;
let editorRevision = 0;
const uploadType = ref('public/image');
const { beforeUpload } = useMediaUpload(uploadType);
const uploading = computed(() => fileQueue.value.some((file) => file.status === 'pending' || file.status === 'uploading'));
const uploadGateway = `${import.meta.env.VITE_HOST}/v1/attachment`;
const uploadToken = computed(() =>
  storeUser.userInfo.id
    ? `Bearer ${localStorage.getItem(TOKEN_KEY) || ''}`
    : '',
);

const PRESET_TAGS = [
  'AI/ML', 'Web', 'Mobile', 'Hardware', 'Data',
  'In Progress', 'Completed', 'Recruiting',
  'Beginner', 'Intermediate', 'Advanced',
];
const presetTagOptions = PRESET_TAGS.map((tag) => ({ label: tag, value: tag }));

const editToolbarsExclude: ToolbarNames[] = [
  'mermaid', 'katex', 'image', 'github', 'htmlPreview', 'fullscreen',
];

function openEditor() {
  if (!project.value || !canEdit.value || showEditor.value || submitting.value || following.value) return;
  ++editorRevision;
  editDraft.title = project.value.title;
  editDraft.intro = project.value.intro;
  editDraft.url = project.value.url;
  editDraft.tags = [...project.value.tags];
  // Pre-fill images into upload state
  fileQueue.value = project.value.images.map((url, i) => ({
    id: `existing-${i}`,
    name: url.split('/').pop() || 'image',
    status: 'finished',
    url,
  } as UploadFileInfo));
  uploadedImages.value = project.value.images.map((url, i) => ({
    id: `existing-${i}`,
    content: url,
  }));
  imageUrlInput.value = '';
  linkError.value = '';
  showEditor.value = true;
}

function addImageUrl() {
  if (!showEditor.value || submitting.value || !imageUrlInput.value.trim()) return;
  const url = safeExternalURL(imageUrlInput.value);
  if (!url) return window.$message.warning(t('project.imageUrlInvalid'));
  if (fileQueue.value.length >= 6) {
    window.$message.warning(t('project.imagesMax'));
    return;
  }
  const id = `url-${++nextImageID}`;
  fileQueue.value.push({
    id,
    name: url.split('/').pop() || 'image',
    status: 'finished',
    url,
  } as UploadFileInfo);
  uploadedImages.value.push({ id, content: url });
  imageUrlInput.value = '';
}

const finishUpload: UploadOnFinish = ({ file, event }) => {
  if (
    !showEditor.value ||
    !fileQueue.value.some((current) => current.id === file.id)
  ) return file;
  try {
    const request = event?.target as XMLHttpRequest | null;
    const data = JSON.parse(request?.response || '{}');
    if (data.code !== 0 || !data.data?.content) throw new Error();
    uploadedImages.value = uploadedImages.value.filter((image) => image.id !== file.id);
    uploadedImages.value.push({ id: file.id, content: data.data.content });
    return file;
  } catch {
    file.status = 'error';
    window.$message.error(t('compose.uploadFailed'));
    return file;
  }
};

function failUpload() {
  window.$message.error(t('compose.uploadFailed'));
}

function removeUpload({ file }: { file: UploadFileInfo }) {
  if (submitting.value) return false;
  uploadedImages.value = uploadedImages.value.filter((item) => item.id !== file.id);
  return true;
}

function updateTags(tags: string[]) {
  if (tags.length > 8) return window.$message.warning(t('project.tagsMax'));
  editDraft.tags = tags;
}

function safeExternalURL(value: string) {
  try {
    const parsed = new URL(value.trim());
    return ['http:', 'https:'].includes(parsed.protocol) ? parsed.href : '';
  } catch {
    return '';
  }
}

function validateLink() {
  linkError.value = safeExternalURL(editDraft.url) ? '' : t('project.linkInvalid');
  return !linkError.value;
}

function resetEditor() {
  ++editorRevision;
  submitting.value = false;
  Object.assign(editDraft, { title: '', intro: '', url: '', tags: [] });
  fileQueue.value = [];
  uploadedImages.value = [];
  imageUrlInput.value = '';
  linkError.value = '';
  uploadRef.value?.clear();
}

async function submitEdit() {
  if (submitting.value || uploading.value || !project.value || !showEditor.value || !canEdit.value) return;
  if (!editDraft.title.trim()) return window.$message.warning(t('project.titleRequired'));
  const tags = [...new Set(editDraft.tags.map((tag) => tag.trim()).filter(Boolean))];
  if (tags.length > 8) return window.$message.warning(t('project.tagsMax'));
  if (!editDraft.intro.trim()) return window.$message.warning(t('project.introRequired'));
  const orderedImages = fileQueue.value
    .map((file) => uploadedImages.value.find((image) => image.id === file.id))
    .filter((image): image is { id: string; content: string } => Boolean(image))
    .map((image) => image.content);
  if (orderedImages.length === 0) return window.$message.warning(t('project.imagesRequired'));
  if (!validateLink()) return;
  const revision = editorRevision;
  const id = project.value.id;
  submitting.value = true;
  try {
    const result = await updateProject({
      id,
      title: editDraft.title.trim(),
      intro: editDraft.intro.trim(),
      url: safeExternalURL(editDraft.url),
      images: orderedImages,
      tags,
    });
    if (revision !== editorRevision || projectID.value !== id || project.value?.id !== id) return;
    project.value = result;
    showEditor.value = false;
    window.$message.success(t('project.saveSuccess'));
  } catch {
    // The request interceptor reports the error; keep the draft open for retry.
  } finally {
    if (revision === editorRevision) submitting.value = false;
  }
}

watch(showEditor, (show) => {
  if (!show) resetEditor();
}, { flush: 'sync' });
onBeforeUnmount(() => {
  ++loadRequestRevision;
  ++followRequestRevision;
  showEditor.value = false;
  resetEditor();
});

watch(projectID, () => void loadProject(), { immediate: true });
</script>

<style scoped lang="less">
.project-detail { box-sizing: border-box; min-height: 520px; padding: 28px; border: 1px solid rgba(128, 128, 128, .2); border-top: 0; }
.detail-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; margin-bottom: 24px; }
.creator { display: flex; align-items: center; gap: 9px; font-size: 14px; }
.creator a { color: inherit; text-decoration: none; opacity: .72; }
.detail-heading { min-width: 0; }
.detail-heading h1 { overflow-wrap: anywhere; margin: 12px 0; font-size: clamp(28px, 5vw, 44px); line-height: 1.15; }
.detail-tags, .project-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.detail-gallery { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 4px; max-height: 620px; overflow: auto; border-radius: 14px; background: rgba(128, 128, 128, .1); }
.detail-gallery.single { display: block; }
.detail-gallery :deep(.n-image), .detail-gallery :deep(img) { display: block; width: 100%; max-height: 440px; }
.detail-gallery :deep(img) { object-fit: cover; }
.detail-columns { display: grid; grid-template-columns: minmax(0, 1fr) 260px; gap: 24px; margin-top: 28px; }
.project-readme, .side-card { border: 1px solid rgba(128, 128, 128, .2); border-radius: 12px; background: rgba(128, 128, 128, .035); }
.project-readme { padding: 24px; }
.section-kicker { color: #18a058; font-size: 12px; font-weight: 700; letter-spacing: .1em; }
.project-readme :deep(.md-editor) { --md-bk-color: transparent; }
.project-readme :deep(.md-editor-preview-wrapper) { padding: 12px 0 0; }
.side-card { padding: 16px; }
.side-card + .side-card { margin-top: 14px; }
.side-card h2 { margin: 0 0 14px; font-size: 16px; }
.member-row > span { min-width: 0; overflow-wrap: anywhere; }
.member-row { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 8px; margin-top: 10px; }
.project-editor-modal { width: min(680px, calc(100vw - 28px)); max-height: calc(100vh - 28px); overflow: auto; }
.project-markdown-editor { height: 320px; }
.tags-select, .upload-field { width: 100%; }
.upload-field small { display: block; margin-top: 8px; opacity: .6; }
.url-input-row { display: flex; gap: 8px; margin-top: 10px; }
.url-input { flex: 1; min-width: 0; }
.editor-actions { display: flex; justify-content: flex-end; gap: 10px; }
.detail-links { display: flex; flex-wrap: wrap; gap: 10px; margin-bottom: 24px; }
@media (max-width: 760px) {
  .project-detail { padding: 18px; }
  .detail-header { flex-direction: column; }
  .detail-columns { grid-template-columns: 1fr; }
  .detail-gallery { grid-template-columns: 1fr; }
}
</style>
