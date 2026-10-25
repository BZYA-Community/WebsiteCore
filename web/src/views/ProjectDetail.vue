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
              <n-button :type="project.followed ? 'default' : 'primary'" :loading="following" @click="toggleFollow">
                <template #icon><n-icon><heart v-if="project.followed" /><heart-outline v-else /></n-icon></template>
                {{ project.followed ? t('project.following') : t('project.follow') }} · {{ project.follow_count }}
              </n-button>
              <n-button secondary @click="joinProject">
                <template #icon><n-icon><people-outline /></n-icon></template>
                {{ t('project.join') }}
              </n-button>
            </div>
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
              <section class="side-card side-actions">
                <n-button type="primary" tag="a" :href="project.url" target="_blank" rel="noopener noreferrer">
                  <template #icon><n-icon><open-outline /></n-icon></template>
                  {{ t('project.visitProject') }}
                </n-button>
                <n-button v-if="project.post_id" @click="openDiscussion">
                  <template #icon><n-icon><chatbox-outline /></n-icon></template>
                  {{ t('project.viewDiscussion') }}
                </n-button>
              </section>
            </aside>
          </div>
        </template>
      </n-spin>
    </main>
  </div>
</template>

<script setup lang="ts">
import { followProject, getProject, unfollowProject } from '@/api/project';
import type { ProjectItem } from '@/api/project';
import { useStoreMain } from '@/store/main';
import { useStoreUser } from '@/store/user';
import { mdTheme, prepareMdRender } from '@/utils/markdown';
import { ChatboxOutline, Heart, HeartOutline, OpenOutline, PeopleOutline } from '@vicons/ionicons5';
import { MdPreview } from 'md-editor-v3';
import { computed, ref, watch } from 'vue';
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
  if (!project.value || following.value) return;
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

function joinProject() { window.$message.info(t('project.joinComingSoon')); }
function openDiscussion() {
  if (project.value?.post_id) void router.push({ name: 'post', query: { id: project.value.post_id } });
}

watch(projectID, () => void loadProject(), { immediate: true });
</script>

<style scoped lang="less">
.project-detail { box-sizing: border-box; min-height: 520px; padding: 28px; border: 1px solid rgba(128, 128, 128, .2); border-top: 0; }
.detail-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 24px; margin-bottom: 24px; }
.creator { display: flex; align-items: center; gap: 9px; font-size: 14px; }
.creator a { color: inherit; text-decoration: none; opacity: .72; }
.detail-heading h1 { margin: 12px 0; font-size: clamp(28px, 5vw, 44px); line-height: 1.15; }
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
.member-row { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 8px; margin-top: 10px; }
.side-actions { display: flex; flex-direction: column; gap: 10px; }
@media (max-width: 760px) {
  .project-detail { padding: 18px; }
  .detail-header { flex-direction: column; }
  .detail-columns { grid-template-columns: 1fr; }
  .detail-gallery { grid-template-columns: 1fr; }
}
</style>
