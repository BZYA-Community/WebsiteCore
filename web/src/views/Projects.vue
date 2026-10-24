<template>
  <div class="projects-page">
    <main-nav :title="t('project.pageTitle')" />

    <section class="project-hero main-content-wrap">
      <div>
        <span class="eyebrow">{{ t('project.eyebrow') }}</span>
        <h1>{{ t('project.heroTitle') }}</h1>
        <p>{{ t('project.heroDescription') }}</p>
      </div>
      <n-button type="primary" size="large" round @click="startPublishing">
        <template #icon><n-icon><add-outline /></n-icon></template>
        {{ t('project.publishProject') }}
      </n-button>
    </section>

    <main class="project-content main-content-wrap">
      <div class="project-toolbar">
        <div>
          <h2>{{ t('project.latestProjects') }}</h2>
          <span>{{ t('project.projectCount', { count: total }) }}</span>
        </div>
        <n-button quaternary circle :loading="loading" :title="t('project.refresh')" @click="loadProjects">
          <template #icon><n-icon><refresh-outline /></n-icon></template>
        </n-button>
      </div>

      <div v-if="loading && projects.length === 0" class="project-grid">
        <n-card v-for="item in 4" :key="item" class="project-card"><n-skeleton height="220px" /></n-card>
      </div>

      <n-result v-else-if="loadFailed" status="error" :title="t('project.loadFailed')">
        <template #footer><n-button @click="loadProjects">{{ t('project.retry') }}</n-button></template>
      </n-result>

      <n-empty v-else-if="projects.length === 0" :description="t('project.empty')" class="project-empty">
        <template #extra><n-button type="primary" @click="startPublishing">{{ t('project.publishFirst') }}</n-button></template>
      </n-empty>

      <div v-else class="project-grid">
        <article v-for="project in projects" :key="project.post.id" class="project-card">
          <div class="project-gallery" :class="{ single: project.images.length === 1 }">
            <n-image
              v-for="(image, index) in project.images.slice(0, 4)"
              :key="image + index"
              :src="image"
              object-fit="cover"
              :alt="project.title"
              lazy
            />
            <span v-if="project.images.length > 4" class="more-images">+{{ project.images.length - 4 }}</span>
          </div>
          <div class="project-body">
            <div class="project-author">
              <n-avatar round :size="28" :src="project.post.user.avatar" />
              <span>{{ project.post.user.nickname }}</span>
            </div>
            <h3>{{ project.title }}</h3>
            <p>{{ project.intro }}</p>
            <div class="project-actions">
              <n-button text @click="openDiscussion(project.post.id)">{{ t('project.viewDiscussion') }}</n-button>
              <n-button v-if="project.url" type="primary" tag="a" :href="project.url" target="_blank" rel="noopener noreferrer">
                {{ t('project.visitProject') }}
                <template #icon><n-icon><open-outline /></n-icon></template>
              </n-button>
            </div>
          </div>
        </article>
      </div>

      <n-pagination
        v-if="total > pageSize"
        v-model:page="page"
        :page-size="pageSize"
        :item-count="total"
        class="project-pagination"
        @update:page="loadProjects"
      />
    </main>

    <n-modal v-model:show="showEditor" preset="card" :title="t('project.editorTitle')" class="project-editor" :mask-closable="!submitting">
      <n-form label-placement="top" @submit.prevent="submitProject">
        <n-form-item :label="t('project.titleLabel')" required>
          <n-input v-model:value="draft.title" maxlength="80" show-count :placeholder="t('project.titlePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('project.introLabel')" required>
          <n-input v-model:value="draft.intro" type="textarea" :autosize="{ minRows: 4, maxRows: 8 }" maxlength="800" show-count :placeholder="t('project.introPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('project.imagesLabel')" required>
          <div class="upload-field">
            <n-upload
              ref="uploadRef"
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
              @update:file-list="updateUpload"
            >
              {{ t('project.uploadImage') }}
            </n-upload>
            <small>{{ t('project.imagesHelp') }}</small>
          </div>
        </n-form-item>
        <n-form-item :label="t('project.linkLabel')" required :validation-status="linkError ? 'error' : undefined" :feedback="linkError">
          <n-input v-model:value="draft.url" type="url" placeholder="https://" @blur="validateLink">
            <template #prefix><n-icon><link-outline /></n-icon></template>
          </n-input>
        </n-form-item>
        <div class="editor-actions">
          <n-button :disabled="submitting" @click="showEditor = false">{{ t('common.cancel') }}</n-button>
          <n-button attr-type="submit" type="primary" :loading="submitting" :disabled="uploading">{{ t('project.publish') }}</n-button>
        </div>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { createPost, getPosts } from '@/api/post';
import { useMediaUpload } from '@/composables/useMediaUpload';
import { useStoreMain } from '@/store/main';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { PostItemTypeEnum, VisibilityEnum } from '@/utils/IEnum';
import {
  AddOutline,
  LinkOutline,
  OpenOutline,
  RefreshOutline,
} from '@vicons/ionicons5';
import type { UploadFileInfo, UploadInst, UploadOnFinish } from 'naive-ui';
import { computed, onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

const PROJECT_TAG = '项目';
const pageSize = 12;
const { t } = useI18n();
const router = useRouter();
const storeUser = useStoreUser();
const storeMain = useStoreMain();
const projects = ref<ProjectView[]>([]);
const page = ref(1);
const total = ref(0);
const loading = ref(false);
const loadFailed = ref(false);
const showEditor = ref(false);
const submitting = ref(false);
const uploadRef = ref<UploadInst>();
const fileQueue = ref<UploadFileInfo[]>([]);
const uploadedImages = ref<{ id: string; content: string }[]>([]);
const draft = reactive({ title: '', intro: '', url: '' });
const linkError = ref('');
const uploadType = ref('public/image');
const { beforeUpload } = useMediaUpload(uploadType);
const uploadGateway = `${import.meta.env.VITE_HOST}/v1/attachment`;
const uploadToken = computed(() =>
  storeUser.userInfo.id
    ? `Bearer ${localStorage.getItem(TOKEN_KEY) || ''}`
    : '',
);
const uploading = computed(() =>
  fileQueue.value.some(
    (file) => file.status === 'pending' || file.status === 'uploading',
  ),
);

interface ProjectView {
  post: Item.PostProps;
  title: string;
  intro: string;
  images: string[];
  url: string;
}

function safeExternalURL(value: string) {
  try {
    const parsed = new URL(value.trim());
    return ['http:', 'https:'].includes(parsed.protocol) ? parsed.href : '';
  } catch {
    return '';
  }
}

function toProject(post: Item.PostProps): ProjectView {
  const sorted = [...post.contents].sort((a, b) => a.sort - b.sort);
  return {
    post,
    title:
      sorted.find((item) => item.type === PostItemTypeEnum.TITLE)?.content ||
      t('project.untitled'),
    intro:
      sorted.find((item) => item.type === PostItemTypeEnum.TEXT)?.content || '',
    images: sorted
      .filter((item) => item.type === PostItemTypeEnum.IMAGEURL)
      .map((item) => item.content),
    url: safeExternalURL(
      sorted.find((item) => item.type === PostItemTypeEnum.LINKURL)?.content ||
        '',
    ),
  };
}

async function loadProjects() {
  loading.value = true;
  loadFailed.value = false;
  try {
    const result = await getPosts({
      query: PROJECT_TAG,
      type: 'tag',
      style: 'newest',
      page: page.value,
      page_size: pageSize,
    });
    projects.value = (result.list || []).map(toProject);
    total.value = result.pager.total_rows;
    window.scrollTo({ top: 0, behavior: 'smooth' });
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}

function startPublishing() {
  if (!storeUser.userLogined) {
    storeMain.triggerAuth(true);
    storeMain.triggerAuthKey('signin');
    return;
  }
  if (
    !storeUser.hasPermission('post.create') ||
    !storeUser.hasPermission('content.upload')
  ) {
    window.$message.warning(t('project.noPermission'));
    return;
  }
  showEditor.value = true;
}

function validateLink() {
  linkError.value =
    draft.url.trim() && safeExternalURL(draft.url)
      ? ''
      : t('project.linkInvalid');
  return !linkError.value;
}

function updateUpload(list: UploadFileInfo[]) {
  fileQueue.value = list;
}
const finishUpload: UploadOnFinish = ({ file, event }) => {
  try {
    const request = event?.target as XMLHttpRequest | null;
    const data = JSON.parse(request?.response || '{}');
    if (data.code !== 0 || !data.data?.content) throw new Error();
    uploadedImages.value.push({ id: file.id, content: data.data.content });
    return file;
  } catch {
    window.$message.error(t('compose.uploadFailed'));
    return file;
  }
};
function failUpload() {
  window.$message.error(t('compose.uploadFailed'));
}
function removeUpload({ file }: { file: UploadFileInfo }) {
  const index = uploadedImages.value.findIndex((item) => item.id === file.id);
  if (index >= 0) uploadedImages.value.splice(index, 1);
  return true;
}

function resetDraft() {
  Object.assign(draft, { title: '', intro: '', url: '' });
  linkError.value = '';
  uploadedImages.value = [];
  fileQueue.value = [];
  uploadRef.value?.clear();
}

async function submitProject() {
  if (submitting.value || uploading.value) return;
  if (!draft.title.trim())
    return window.$message.warning(t('project.titleRequired'));
  if (!draft.intro.trim())
    return window.$message.warning(t('project.introRequired'));
  if (uploadedImages.value.length === 0)
    return window.$message.warning(t('project.imagesRequired'));
  if (!validateLink()) return;
  const orderedImages = fileQueue.value
    .map((file) => uploadedImages.value.find((image) => image.id === file.id))
    .filter((image): image is { id: string; content: string } =>
      Boolean(image),
    );
  let sort = 100;
  const contents: Partial<Item.PostItemProps>[] = [
    { content: draft.title.trim(), type: PostItemTypeEnum.TITLE, sort: sort++ },
    { content: draft.intro.trim(), type: PostItemTypeEnum.TEXT, sort: sort++ },
    ...orderedImages.map((image) => ({
      content: image.content,
      type: PostItemTypeEnum.IMAGEURL,
      sort: sort++,
    })),
    {
      content: safeExternalURL(draft.url),
      type: PostItemTypeEnum.LINKURL,
      sort,
    },
  ];
  submitting.value = true;
  try {
    const result = await createPost({
      contents,
      tags: [PROJECT_TAG],
      users: [],
      visibility: VisibilityEnum.PUBLIC,
    });
    window.$message.success(
      result.audit_status === 0
        ? t('compose.publishSuccessAudit')
        : t('project.publishSuccess'),
    );
    showEditor.value = false;
    resetDraft();
    page.value = 1;
    await loadProjects();
  } finally {
    submitting.value = false;
  }
}

function openDiscussion(id: number) {
  void router.push({ name: 'post', query: { id } });
}
onMounted(loadProjects);
</script>

<style scoped lang="less">
.projects-page { --project-accent: #18a058; --project-surface: var(--n-color, #fff); }
.project-hero { box-sizing: border-box; display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; padding: 34px 30px; border: 1px solid rgba(128, 128, 128, .2); border-top: 0; background: radial-gradient(circle at 85% 5%, color-mix(in srgb, var(--project-accent) 20%, transparent), transparent 36%); }
.eyebrow { color: var(--project-accent); font-size: 12px; font-weight: 700; letter-spacing: .12em; text-transform: uppercase; }
.project-hero h1 { margin: 8px 0; font-size: clamp(25px, 4vw, 38px); line-height: 1.15; }
.project-hero p { margin: 0; max-width: 58ch; opacity: .68; line-height: 1.7; }
.project-content { box-sizing: border-box; min-height: 440px; padding: 24px; border: 1px solid rgba(128, 128, 128, .2); border-top: 0; }
.project-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.project-toolbar h2 { margin: 0 0 4px; font-size: 20px; }
.project-toolbar span { font-size: 13px; opacity: .58; }
.project-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; }
.project-card { overflow: hidden; border: 1px solid rgba(128, 128, 128, .2); border-radius: 14px; background: var(--project-surface); transition: transform .2s ease, box-shadow .2s ease; }
:global(.dark) .project-card { background: rgba(16, 16, 20, .78); }
.project-card:hover { transform: translateY(-2px); box-shadow: 0 14px 32px rgba(0, 0, 0, .08); }
.project-gallery { position: relative; display: grid; grid-template-columns: 1fr 1fr; grid-template-rows: 110px 110px; gap: 2px; background: rgba(128, 128, 128, .12); }
.project-gallery.single { display: block; height: 222px; }
.project-gallery :deep(.n-image), .project-gallery :deep(img) { width: 100%; height: 100%; display: block; }
.project-gallery :deep(img) { object-fit: cover; }
.more-images { position: absolute; right: 10px; bottom: 10px; padding: 4px 9px; border-radius: 14px; color: #fff; background: rgba(0, 0, 0, .65); font-weight: 600; }
.project-body { padding: 18px; }
.project-author { display: flex; align-items: center; gap: 8px; font-size: 12px; opacity: .7; }
.project-body h3 { margin: 14px 0 8px; font-size: 20px; line-height: 1.35; overflow-wrap: anywhere; }
.project-body > p { min-height: 3.4em; margin: 0; opacity: .7; line-height: 1.7; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.project-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 18px; }
.project-empty { padding: 70px 0; }
.project-pagination { justify-content: center; margin-top: 28px; }
.project-editor { width: min(680px, calc(100vw - 28px)); }
.upload-field { width: 100%; }
.upload-field small { display: block; margin-top: 8px; opacity: .6; }
.editor-actions { display: flex; justify-content: flex-end; gap: 10px; }
@media (max-width: 720px) {
  .project-hero { align-items: flex-start; flex-direction: column; padding: 26px 20px; }
  .project-content { padding: 16px; }
  .project-grid { grid-template-columns: 1fr; }
}
</style>
