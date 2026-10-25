<template>
  <article class="project-card">
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
        <n-avatar round :size="28" :src="project.creator.avatar" />
        <span>{{ project.creator.nickname }}</span>
      </div>
      <h3>{{ project.title }}</h3>
      <div class="project-tags">
        <n-tag
          v-for="tag in project.tags"
          :key="tag"
          size="small"
          round
          :clickable="tagClickable"
          @click="handleTagClick(tag)"
        >
          {{ tag }}
        </n-tag>
      </div>
      <div class="project-intro" @click="handleMarkdownClick">
        <md-preview :model-value="prepareMdRender(project.intro)" :theme="editorTheme" no-mermaid no-katex no-html />
      </div>
      <div class="project-actions">
        <n-button text @click="openProject">{{ t('project.viewDetails') }}</n-button>
        <n-button v-if="project.url" type="primary" tag="a" :href="project.url" target="_blank" rel="noopener noreferrer">
          {{ t('project.visitProject') }}
          <template #icon><n-icon><open-outline /></n-icon></template>
        </n-button>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import type { ProjectItem } from '@/api/project';
import { useStoreMain } from '@/store/main';
import { useStoreProfile } from '@/store/profile';
import { openExternalLink } from '@/utils/external-link';
import { mdTheme, prepareMdRender } from '@/utils/markdown';
import { OpenOutline } from '@vicons/ionicons5';
import { MdPreview } from 'md-editor-v3';
import 'md-editor-v3/lib/style.css';
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

const props = withDefaults(defineProps<{
  project: ProjectItem;
  tagClickable?: boolean;
}>(), {
  tagClickable: false,
});
const emit = defineEmits<{
  tagClick: [tag: string];
}>();
const { t } = useI18n();
const router = useRouter();
const storeMain = useStoreMain();
const storeProfile = useStoreProfile();
const editorTheme = computed(() => mdTheme(storeMain.theme));

function handleTagClick(tag: string) {
  if (props.tagClickable) emit('tagClick', tag);
}

function handleMarkdownClick(event: MouseEvent) {
  const anchor = (event.target as HTMLElement).closest('a');
  const href = anchor?.getAttribute('href') || '';
  if (!anchor || (!href.startsWith('http://') && !href.startsWith('https://'))) return;
  if (openExternalLink(router, href, {
    trustedDomains: storeProfile.profile.trustedExternalDomains,
  })) event.preventDefault();
}

function openProject() {
  void router.push({ name: 'project', query: { id: props.project.id } });
}
</script>

<style lang="less" scoped>
.project-card {
  overflow: hidden;
  border: 1px solid rgba(128, 128, 128, .2);
  border-radius: 14px;
  background: var(--project-surface, rgba(255, 255, 255, .94));
  transition: transform .2s ease, box-shadow .2s ease;
}
:global(.dark .project-card) { background: rgba(16, 16, 20, .78); }
.project-card:hover { transform: translateY(-2px); box-shadow: 0 14px 32px rgba(0, 0, 0, .08); }
.project-gallery { position: relative; display: grid; grid-template-columns: 1fr 1fr; grid-template-rows: 110px 110px; gap: 2px; background: rgba(128, 128, 128, .12); }
.project-gallery.single { display: block; height: 222px; }
.project-gallery :deep(.n-image), .project-gallery :deep(img) { width: 100%; height: 100%; display: block; }
.project-gallery :deep(img) { object-fit: cover; }
.more-images { position: absolute; right: 10px; bottom: 10px; padding: 4px 9px; border-radius: 14px; color: #fff; background: rgba(0, 0, 0, .65); font-weight: 600; }
.project-body { padding: 18px; }
.project-author { display: flex; align-items: center; gap: 8px; font-size: 12px; opacity: .7; }
.project-body h3 { margin: 14px 0 8px; font-size: 20px; line-height: 1.35; overflow-wrap: anywhere; }
.project-tags { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px; }
.project-intro { min-height: 3.4em; max-height: 7.2em; overflow: hidden; opacity: .7; line-height: 1.7; }
.project-intro :deep(.md-editor) { --md-bk-color: transparent; }
.project-intro :deep(.md-editor-preview-wrapper) { padding: 0; }
.project-intro :deep(.md-editor-preview) { font-size: 14px; }
.project-intro :deep(.md-editor-preview > :first-child) { margin-top: 0; }
.project-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 18px; }
</style>
