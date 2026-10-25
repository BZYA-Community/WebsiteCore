<template>
  <div>
    <main-nav :title="t('user.collection.title')" />

    <n-list class="main-content-wrap" bordered>
      <n-tabs class="collection-tabs" type="line" animated :value="activeTab" @update:value="changeTab">
        <n-tab-pane name="posts"><template #tab>{{ t('user.collection.tabPosts') }}</template></n-tab-pane>
        <n-tab-pane v-if="projectsEnabled" name="projects"><template #tab>{{ t('user.collection.tabProjects') }}</template></n-tab-pane>
      </n-tabs>

      <template v-if="activeTab === 'posts'">
        <div v-if="postLoading && postList.length === 0" class="skeleton-wrap">
          <post-skeleton :num="postPageSize" />
        </div>
        <n-result v-else-if="postLoadFailed" status="error" :title="t('user.collection.loadError')">
          <template #footer><n-button @click="loadPosts">{{ t('project.retry') }}</n-button></template>
        </n-result>
        <div v-else>
          <div v-if="postList.length === 0" class="empty-wrap">
            <n-empty size="large" :description="t('common.noData')" />
          </div>

          <n-list-item v-for="post in postList" :key="post.id">
            <post-item
              :post="post"
              :is-owner="userInfo.id === post.user_id"
              :is-mobile="!desktopModelShow"
              add-follow-action
              @send-whisper="onSendWhisper"
              @post-follow-action="postFollowAction"
            />
          </n-list-item>
        </div>
      </template>

      <template v-else>
        <div v-if="projectLoading && projectList.length === 0" class="project-grid">
          <n-card v-for="item in 4" :key="item" class="project-skeleton"><n-skeleton height="220px" /></n-card>
        </div>
        <n-result v-else-if="projectLoadFailed" status="error" :title="t('project.loadFailed')">
          <template #footer><n-button @click="loadProjects">{{ t('project.retry') }}</n-button></template>
        </n-result>
        <n-empty v-else-if="projectList.length === 0" class="project-empty" :description="t('user.collection.projectEmpty')" />
        <div v-else class="project-grid">
          <project-card v-for="project in projectList" :key="project.id" :project="project" />
        </div>
      </template>
    </n-list>

    <n-space v-if="activeTab === 'posts' && !postLoadFailed && postTotalPage > 0" justify="center">
      <InfiniteLoading
        class="load-more"
        :slots="{ complete: t('user.collection.noMore'), error: t('user.collection.loadError') }"
        @infinite="nextPostPage"
      >
        <template #spinner>
          <div class="load-more-wrap">
            <n-spin v-if="!postNoMore" :size="14" />
            <span class="load-more-spinner">{{ postNoMore ? t('user.collection.noMore') : t('user.collection.loadMore') }}</span>
          </div>
        </template>
      </InfiniteLoading>
    </n-space>

    <n-pagination
      v-else-if="activeTab === 'projects' && projectTotal > projectPageSize"
      v-model:page="projectPage"
      :page-size="projectPageSize"
      :item-count="projectTotal"
      class="project-pagination"
      @update:page="changeProjectPage"
    />
  </div>
</template>

<script setup lang="ts">
import { getProjects } from '@/api/project';
import type { ProjectItem } from '@/api/project';
import ProjectCard from '@/components/project-card.vue';
import { useChatJump } from '@/composables/useUserAction';
import { useStoreMain } from '@/store/main';
import { useStoreProfile } from '@/store/profile';
import { useStoreUser } from '@/store/user';
import { Api } from '@/utils/request';
import InfiniteLoading from 'v3-infinite-loading';
import { storeToRefs } from 'pinia';
import { computed, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';

type CollectionTab = 'posts' | 'projects';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const storeMain = useStoreMain();
const storeUser = useStoreUser();
const storeProfile = useStoreProfile();
const { desktopModelShow } = storeToRefs(storeMain);
const { userInfo } = storeToRefs(storeUser);
const { profile } = storeToRefs(storeProfile);
const projectsEnabled = computed(() => profile.value.projectsEnabled);
const activeTab = ref<CollectionTab>(
  route.query.tab === 'projects' && projectsEnabled.value ? 'projects' : 'posts',
);

const postList = ref<Item.PostProps[]>([]);
const postLoading = ref(false);
const postLoadFailed = ref(false);
const postNoMore = ref(false);
const postPage = ref(Math.max(1, Number(route.query.p) || 1));
const postPageSize = 20;
const postTotalPage = ref(0);
const postsLoaded = ref(false);
let postRequestRevision = 0;

const projectList = ref<ProjectItem[]>([]);
const projectLoading = ref(false);
const projectLoadFailed = ref(false);
const projectPage = ref(1);
const projectPageSize = 12;
const projectTotal = ref(0);
const projectsLoaded = ref(false);
let projectRequestRevision = 0;

const { goWhisper: onSendWhisper } = useChatJump();

function postFollowAction(userId: number, isFollowing: boolean) {
  for (const post of postList.value) {
    if (post.user_id === userId) post.user.is_following = isFollowing;
  }
}

async function loadPosts() {
  if (postLoading.value) return;
  const revision = ++postRequestRevision;
  postLoading.value = true;
  postLoadFailed.value = false;
  try {
    const result = await Api.v1.user.get.collections({
      page: postPage.value,
      page_size: postPageSize,
    });
    if (revision !== postRequestRevision) return;
    postsLoaded.value = true;
    postNoMore.value = result.list.length === 0;
    postList.value = postPage.value > 1
      ? postList.value.concat(result.list)
      : result.list;
    postTotalPage.value = Math.ceil(result.pager.total_rows / postPageSize);
    if (postPage.value === 1) window.scrollTo(0, 0);
  } catch {
    if (revision === postRequestRevision) {
      if (postPage.value > 1) postPage.value--;
      else postLoadFailed.value = true;
    }
  } finally {
    if (revision === postRequestRevision) postLoading.value = false;
  }
}

function nextPostPage() {
  if (postLoading.value) return;
  if (postPage.value < postTotalPage.value || postTotalPage.value === 0) {
    postNoMore.value = false;
    postPage.value++;
    void loadPosts();
  } else {
    postNoMore.value = true;
  }
}

async function loadProjects() {
  if (!projectsEnabled.value) return;
  const revision = ++projectRequestRevision;
  const requestedPage = projectPage.value;
  projectLoading.value = true;
  projectLoadFailed.value = false;
  try {
    const result = await getProjects({
      followed: true,
      page: projectPage.value,
      page_size: projectPageSize,
    });
    if (revision !== projectRequestRevision) return;
    const lastPage = Math.max(1, Math.ceil(result.pager.total_rows / projectPageSize));
    if (requestedPage > lastPage) {
      projectPage.value = lastPage;
      void loadProjects();
      return;
    }
    projectsLoaded.value = true;
    projectList.value = result.list || [];
    projectTotal.value = result.pager.total_rows;
  } catch {
    if (revision === projectRequestRevision) projectLoadFailed.value = true;
  } finally {
    if (revision === projectRequestRevision) projectLoading.value = false;
  }
}

function changeProjectPage(page: number) {
  projectPage.value = page;
  void loadProjects();
  window.scrollTo({ top: 0, behavior: 'smooth' });
}

function changeTab(value: string) {
  if (value !== 'posts' && value !== 'projects') return;
  if (value === 'projects' && !projectsEnabled.value) return;
  activeTab.value = value;
  void router.replace({
    query: {
      ...route.query,
      tab: value === 'projects' ? 'projects' : undefined,
      p: undefined,
    },
  });
  if (value === 'projects' && !projectsLoaded.value) void loadProjects();
  if (value === 'posts' && !postsLoaded.value) void loadPosts();
}

watch(projectsEnabled, (enabled) => {
  if (!enabled && activeTab.value === 'projects') changeTab('posts');
});

onMounted(() => {
  if (activeTab.value === 'projects') void loadProjects();
  else void loadPosts();
});
</script>

<style lang="less" scoped>
.main-content-wrap { box-sizing: border-box; padding: 0 20px 20px; }
.collection-tabs { margin-bottom: 4px; }
.empty-wrap, .skeleton-wrap { padding: 24px 0; }
.project-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; padding-top: 16px; }
.project-skeleton { overflow: hidden; border-radius: 14px; }
.project-empty { padding: 70px 0; }
.project-pagination { justify-content: center; margin: 24px 0; }
.load-more { margin: 20px; }
.load-more-wrap { display: flex; flex-direction: row; justify-content: center; align-items: center; gap: 14px; }
.load-more-spinner { font-size: 14px; opacity: .65; }
:global(.dark) .main-content-wrap,
:global(.dark) .empty-wrap,
:global(.dark) .skeleton-wrap { background-color: rgba(16, 16, 20, .75); }
@media (max-width: 720px) {
  .main-content-wrap { padding: 0 14px 16px; }
  .project-grid { grid-template-columns: 1fr; }
}
</style>
