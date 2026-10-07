<template>
  <div class="course-watch">
    <nav class="watch-breadcrumb" :aria-label="t('courseWatch.breadcrumb')">
      <router-link :to="{ name: 'courses' }">{{ t('courseWatch.courses') }}</router-link>
      <template v-if="course">
        <span aria-hidden="true">/</span>
        <router-link :to="catalogLocation">{{ course.group_name || t('courseWatch.catalog') }}</router-link>
        <span aria-hidden="true">/</span>
        <span aria-current="page">{{ course.title }}</span>
      </template>
    </nav>

    <div v-if="loading" class="page-state" role="status"><n-spin size="large" /><p>{{ t('courseWatch.loading') }}</p></div>
    <div v-else-if="!course" class="page-state">
      <h1>{{ t('courseWatch.courseUnavailable') }}</h1>
      <p>{{ t('courseWatch.courseUnavailableHelp') }}</p>
      <n-button @click="loadCourse">{{ t('courseWatch.retry') }}</n-button>
    </div>

    <template v-else>
      <header class="watch-heading">
        <div>
          <p class="course-name">{{ course.title }}</p>
          <h1>{{ lesson?.title || t('courseWatch.chooseLesson') }}</h1>
        </div>
        <router-link v-if="canManage" class="manage-link" :to="{ ...catalogLocation, query: { ...catalogLocation.query, manage: '1' } }">{{ t('courseWatch.manage') }}</router-link>
      </header>

      <div class="learning-layout">
        <section class="watch-screen" :aria-label="t('courseWatch.player')" :aria-busy="videoLoading">
          <div class="player-stage">
            <video-player v-if="videoUrl && !videoError" :key="`${lesson?.id}-${selectedVideo?.id}-${videoRevision}`" :src="videoUrl" :poster="course.cover" @play="onFirstPlay" @error="videoError = true" />
            <div v-else class="player-state" role="status">
              <template v-if="videoLoading"><n-spin size="large" /><p>{{ t('courseWatch.videoLoading') }}</p></template>
              <template v-else-if="videoError"><h2>{{ t('courseWatch.videoError') }}</h2><p>{{ t('courseWatch.videoErrorHelp') }}</p><n-button strong secondary @click="loadVideo">{{ t('courseWatch.retryVideo') }}</n-button></template>
              <template v-else-if="!lesson"><h2>{{ lessons.length ? t('courseWatch.lessonUnavailable') : t('courseWatch.noLessons') }}</h2><p>{{ lessons.length ? t('courseWatch.chooseFromList') : t('courseWatch.noLessonsHelp') }}</p></template>
              <template v-else><h2>{{ t('courseWatch.noVideo') }}</h2><p>{{ t('courseWatch.noVideoHelp') }}</p><n-button v-if="lesson.attachments.length" strong secondary @click="showAttachments">{{ t('courseWatch.viewAttachments') }}</n-button></template>
            </div>
          </div>
          <div v-if="videos.length > 1" class="video-options" :aria-label="t('courseWatch.videos')">
            <span>{{ t('courseWatch.videos') }}</span>
            <button v-for="file in videos" :key="file.id" type="button" :title="file.name" :class="{ selected: file.id === selectedVideo?.id }" :aria-pressed="file.id === selectedVideo?.id" @click="selectVideo(file)">{{ file.name }}</button>
          </div>
          <div class="lesson-navigation">
            <n-button :disabled="lessonIndex <= 0" @click="goLesson(lessonIndex - 1)">{{ t('courseWatch.previous') }}</n-button>
            <span v-if="lesson">{{ t('courseWatch.progress', { current: lessonIndex + 1, total: lessons.length }) }}</span>
            <n-button :disabled="lessonIndex < 0 || lessonIndex >= lessons.length - 1" @click="goLesson(lessonIndex + 1)">{{ t('courseWatch.next') }}</n-button>
          </div>
        </section>

        <aside class="lesson-sidebar" :aria-label="t('courseWatch.outline')">
          <div class="outline-heading"><h2>{{ t('courseWatch.outline') }}</h2><span>{{ t('courseWatch.lessonCount', { count: lessons.length }) }}</span></div>
          <ol v-if="lessons.length" ref="playlist" class="lesson-playlist">
            <li v-for="(item, index) in lessons" :key="item.id">
              <router-link :to="lessonLocation(item.id)" :class="{ active: item.id === lesson?.id }" :aria-current="item.id === lesson?.id ? 'true' : undefined">
                <span class="lesson-number">{{ String(index + 1).padStart(2, '0') }}</span>
                <span class="playlist-text"><span class="playlist-title">{{ item.title }}</span><span v-if="item.id === lesson?.id" class="playlist-status">{{ t('courseWatch.currentLesson') }}</span><span v-else class="playlist-status">{{ item.attachments.some(isVideo) ? t('courseWatch.videoLesson') : t('courseWatch.readingLesson') }}</span></span>
                <svg v-if="item.id === lesson?.id" class="playing-indicator" width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M4 2.5v11L13 8z" /></svg>
              </router-link>
            </li>
          </ol>
          <p v-else class="outline-empty">{{ t('courseWatch.noLessons') }}</p>
        </aside>
      </div>

      <section ref="details" class="lesson-details">
        <div class="detail-tabs" role="tablist" :aria-label="t('courseWatch.lessonDetails')" @keydown="onTabKeydown">
          <button v-for="tab in detailTabs" :id="`course-tab-${tab.id}`" :key="tab.id" type="button" role="tab" :aria-controls="`course-panel-${tab.id}`" :aria-selected="activeTab === tab.id" :tabindex="activeTab === tab.id ? 0 : -1" @click="activeTab = tab.id">{{ tab.label }}</button>
        </div>
          <div v-show="activeTab === 'intro'" id="course-panel-intro" role="tabpanel" aria-labelledby="course-tab-intro" tabindex="0">
            <div class="intro-layout">
              <div class="lesson-copy"><h2>{{ t('courseWatch.lessonIntro') }}</h2><p>{{ lesson?.intro || t('courseWatch.noIntro') }}</p><template v-if="course.intro"><h2>{{ t('courseWatch.aboutCourse') }}</h2><p>{{ course.intro }}</p></template></div>
              <div class="teacher-panel">
                <h2>{{ t('courseWatch.teacher') }}</h2>
                <router-link v-if="course.teacher" class="teacher-link" :to="{ name: 'user', query: { s: course.teacher.username } }"><n-avatar round :size="42" :src="course.teacher.avatar" /><span>{{ course.teacher.nickname || course.teacher.username }}</span></router-link>
                <p>{{ course.teacher_intro || t('courseWatch.noTeacherIntro') }}</p>
              </div>
            </div>
          </div>
          <div v-show="activeTab === 'attachments'" id="course-panel-attachments" role="tabpanel" aria-labelledby="course-tab-attachments" tabindex="0">
            <h2>{{ t('courseWatch.lessonAttachments') }}</h2>
            <ul v-if="lesson?.attachments.length" class="file-list">
              <li v-for="file in lesson.attachments" :key="file.id"><span class="file-info"><strong>{{ file.name }}</strong><span>{{ formatSize(file.file_size) }}<template v-if="isVideo(file)"> · {{ t('courseWatch.videoFile') }}</template></span></span><n-button :loading="downloadingFile === file.id" :disabled="downloadingFile !== 0" @click="downloadFile(file)">{{ t('courseWatch.openFile') }}</n-button></li>
            </ul>
            <p v-else class="empty-copy">{{ t('courseWatch.noAttachments') }}</p>
            <n-alert v-if="downloadError" type="error" class="download-error">{{ downloadError }}</n-alert>
          </div>
          <div v-show="activeTab === 'questions'" id="course-panel-questions" role="tabpanel" aria-labelledby="course-tab-questions" tabindex="0">
            <div class="question-heading"><h2>{{ t('courseWatch.courseQuestions') }}</h2><p>{{ t('courseWatch.questionsHelp') }}</p></div>
            <n-alert type="info" class="review-notice">{{ t('course.detail.reviewNotice') }}</n-alert>
            <course-compose-comment :key="course.id" :course-id="course.id" @post-success="reloadComments" />
            <div class="question-list"><course-comment-item v-for="comment in comments" :key="comment.id" :comment="comment" @reload="reloadComments" /></div>
            <p v-if="!commentLoading && !commentError && !comments.length" class="empty-copy">{{ t('course.detail.noComments') }}</p>
            <n-alert v-if="commentError" type="error">{{ t('courseWatch.questionsError') }}</n-alert>
            <div v-if="commentLoading || commentError || hasMoreComments" class="load-comments"><n-button :loading="commentLoading" @click="loadComments">{{ commentError ? t('courseWatch.retry') : t('courseWatch.moreQuestions') }}</n-button></div>
          </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRoute, useRouter } from 'vue-router';
import VideoPlayer from '@/components/video-player.vue';
import { useStoreUser } from '@/store/user';
import { getCourse, getCourseLessons, getCourseAttachment, getCourseComments, playCourse, type CourseItem, type CourseLesson, type LessonAttachment, type CourseComment } from '@/api/course';

const { t } = useI18n();
const route = useRoute();
const router = useRouter();
const storeUser = useStoreUser();
const courseId = computed(() => Number(route.query.id));
const loading = ref(true);
const course = ref<CourseItem | null>(null);
const lessons = ref<CourseLesson[]>([]);
const lesson = computed(() => lessons.value.find((item) => item.id === Number(route.query.lesson)) || null);
const lessonIndex = computed(() => lessons.value.findIndex((item) => item.id === lesson.value?.id));
const canManage = computed(() => storeUser.hasPermission('course.manage') || (storeUser.hasPermission('course.manage_own') && course.value?.teacher_id === storeUser.userInfo.id));
const catalogLocation = computed(() => ({ name: 'courses', query: { category: String(course.value?.group_id || '') } }));
const lessonLocation = (id: number) => ({ name: 'course', query: { id: String(courseId.value), lesson: String(id) } });
const isVideo = (file: LessonAttachment) => file.mime_type?.startsWith('video/');
const videos = computed(() => lesson.value?.attachments.filter(isVideo) || []);
const selectedVideo = ref<LessonAttachment>();
const videoUrl = ref('');
const videoLoading = ref(false);
const videoError = ref(false);
const videoRevision = ref(0);
const activeTab = ref('intro');
const detailTabs = computed(() => [
  { id: 'intro', label: t('courseWatch.introduction') },
  { id: 'attachments', label: t('courseWatch.attachments', { count: lesson.value?.attachments.length || 0 }) },
  { id: 'questions', label: t('courseWatch.questions') },
]);
const details = ref<HTMLElement>();
const playlist = ref<HTMLOListElement>();
let courseRevision = 0;
let playCounted = false;

async function loadCourse() {
  const revision = ++courseRevision;
  const id = courseId.value;
  loading.value = true;
  course.value = null;
  lessons.value = [];
  activeTab.value = 'intro';
  playCounted = false;
  resetComments();
  try {
    if (!Number.isSafeInteger(id) || id <= 0) return;
    const [result, outline] = await Promise.all([getCourse({ id }), getCourseLessons(id)]);
    if (revision !== courseRevision) return;
    course.value = result.course;
    lessons.value = outline.lessons || [];
    if (!route.query.lesson && lessons.value[0]) await router.replace(lessonLocation(lessons.value[0].id));
  } catch { /* The page provides a retry action in addition to the request error. */ }
  finally { if (revision === courseRevision) loading.value = false; }
}

function goLesson(index: number) { if (lessons.value[index]) router.push(lessonLocation(lessons.value[index].id)); }
function safeSignedUrl(value: string) {
  if (!value.trim()) throw new Error('Missing resource URL');
  const url = new URL(value, window.location.origin);
  if (!['http:', 'https:'].includes(url.protocol)) throw new Error('Invalid resource URL');
  return url.href;
}
async function loadVideo() {
  const revision = ++videoRevision.value;
  videoUrl.value = '';
  videoError.value = false;
  videoLoading.value = false;
  const file = selectedVideo.value;
  if (!file) return;
  videoLoading.value = true;
  try {
    const result = await getCourseAttachment(file.id);
    if (revision === videoRevision.value) videoUrl.value = safeSignedUrl(result.signed_url);
  } catch { if (revision === videoRevision.value) videoError.value = true; }
  finally { if (revision === videoRevision.value) videoLoading.value = false; }
}
function selectVideo(file: LessonAttachment) {
  if (selectedVideo.value?.id === file.id) return;
  selectedVideo.value = file;
  loadVideo();
}
function onFirstPlay() {
  if (playCounted || !course.value) return;
  playCounted = true;
  const id = course.value.id;
  playCourse({ id }).then((result) => { if (course.value?.id === id) course.value.play_count = result.play_count; }).catch(() => {});
}
async function showAttachments() {
  activeTab.value = 'attachments';
  await nextTick();
  details.value?.scrollIntoView({ block: 'start' });
}
function onTabKeydown(event: KeyboardEvent) {
  const index = detailTabs.value.findIndex((tab) => tab.id === activeTab.value);
  let next = index;
  if (event.key === 'ArrowRight') next = (index + 1) % detailTabs.value.length;
  else if (event.key === 'ArrowLeft') next = (index + detailTabs.value.length - 1) % detailTabs.value.length;
  else if (event.key === 'Home') next = 0;
  else if (event.key === 'End') next = detailTabs.value.length - 1;
  else return;
  event.preventDefault();
  activeTab.value = detailTabs.value[next].id;
  details.value?.querySelector<HTMLButtonElement>(`#course-tab-${activeTab.value}`)?.focus();
}

const formatSize = (size: number) => size >= 1024 ** 3 ? `${(size / 1024 ** 3).toFixed(1)} GB` : size >= 1024 ** 2 ? `${(size / 1024 ** 2).toFixed(1)} MB` : size >= 1024 ? `${(size / 1024).toFixed(1)} KB` : `${size} B`;
const downloadingFile = ref(0);
const downloadError = ref('');
let downloadRevision = 0;
async function downloadFile(file: LessonAttachment) {
  if (downloadingFile.value) return;
  // Open during the click to avoid popup blocking; storage requests never receive the session token.
  const tab = window.open('about:blank', '_blank');
  if (!tab) { downloadError.value = t('courseWatch.popupBlocked'); return; }
  tab.opener = null;
  const revision = ++downloadRevision;
  downloadingFile.value = file.id;
  downloadError.value = '';
  try {
    const result = await getCourseAttachment(file.id);
    if (revision === downloadRevision) tab.location.replace(safeSignedUrl(result.signed_url));
    else tab.close();
  } catch { tab.close(); if (revision === downloadRevision) downloadError.value = t('courseWatch.downloadError'); }
  finally { if (revision === downloadRevision) downloadingFile.value = 0; }
}

const comments = ref<CourseComment[]>([]);
const commentPage = ref(1);
const commentTotal = ref(0);
const commentLoading = ref(false);
const commentError = ref(false);
const commentsLoaded = ref(false);
const hasMoreComments = computed(() => !commentsLoaded.value || comments.value.length < commentTotal.value);
let commentsRevision = 0;
function resetComments() {
  commentsRevision++;
  comments.value = [];
  commentPage.value = 1;
  commentTotal.value = 0;
  commentLoading.value = false;
  commentError.value = false;
  commentsLoaded.value = false;
}
async function loadComments() {
  if (!course.value || commentLoading.value || !hasMoreComments.value) return;
  const revision = commentsRevision;
  commentLoading.value = true;
  commentError.value = false;
  try {
    const result = await getCourseComments({ id: course.value.id, page: commentPage.value, page_size: 20 });
    if (revision !== commentsRevision) return;
    comments.value.push(...(result.list || []));
    commentTotal.value = result.pager?.total_rows || 0;
    commentPage.value++;
    commentsLoaded.value = true;
  } catch { if (revision === commentsRevision) commentError.value = true; }
  finally { if (revision === commentsRevision) commentLoading.value = false; }
}
function reloadComments() { resetComments(); loadComments(); }

watch(courseId, loadCourse, { immediate: true });
watch(lesson, async () => {
  if (!route.query.lesson && course.value?.id === courseId.value && lessons.value[0]) void router.replace(lessonLocation(lessons.value[0].id));
  selectedVideo.value = videos.value[0];
  loadVideo();
  downloadRevision++;
  downloadingFile.value = 0;
  downloadError.value = '';
  await nextTick();
  const list = playlist.value;
  const current = list?.querySelector<HTMLElement>('[aria-current="true"]');
  if (list && current) {
    const top = current.getBoundingClientRect().top - list.getBoundingClientRect().top;
    if (top < 0) list.scrollTop += top;
    else if (top + current.offsetHeight > list.clientHeight) list.scrollTop += top + current.offsetHeight - list.clientHeight;
  }
});
watch(activeTab, (tab) => { if (tab === 'questions' && !commentsLoaded.value) loadComments(); });
onBeforeUnmount(() => { courseRevision++; videoRevision.value++; commentsRevision++; downloadRevision++; });
</script>

<style lang="less" scoped>
.course-watch { box-sizing: border-box; container: course-watch / inline-size; color: var(--course-text, rgb(31, 34, 37)); padding: 16px 16px 24px; font-size: 14px; }
.watch-breadcrumb { display: flex; align-items: baseline; flex-wrap: wrap; gap: 8px; font-size: 12px; color: var(--course-muted, rgb(118, 124, 130)); overflow-wrap: anywhere; a { color: inherit; text-decoration: none; } a:hover { color: var(--course-accent, #18a058); } }
.watch-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin: 16px 0; h1 { margin: 4px 0 0; font-size: 18px; font-weight: 500; line-height: 1.5; overflow-wrap: anywhere; } }
.course-name { margin: 0; font-size: 12px; color: var(--course-muted, rgb(118, 124, 130)); }
.manage-link { flex-shrink: 0; font-size: 13px; color: var(--course-accent, #18a058); text-decoration: none; }
.learning-layout { display: grid; grid-template-columns: minmax(0, 1fr) 240px; gap: 16px; align-items: start; }
.watch-screen { min-width: 0; }
.player-stage { background: #000; border-radius: 3px; overflow: hidden; :deep(.video-player-wrap) { border-radius: 0; } }
.player-state { aspect-ratio: 16 / 9; min-height: 200px; padding: 20px; box-sizing: border-box; display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; color: #fff; h2 { color: inherit; margin: 0; font-size: 16px; font-weight: 500; } p { max-width: 34em; color: rgba(255, 255, 255, .7); line-height: 1.7; margin: 8px 0 16px; } .n-button { color: #fff; background: rgba(255, 255, 255, .15); } }
.video-options { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; padding-top: 12px; font-size: 13px; color: var(--course-muted, rgb(118, 124, 130)); button { max-width: 240px; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; background: var(--course-surface, #fff); border: 1px solid var(--course-line, rgb(239, 239, 245)); border-radius: 3px; padding: 6px 12px; color: var(--course-text, rgb(31, 34, 37)); font: inherit; cursor: pointer; } button.selected { border-color: var(--course-accent, #18a058); color: var(--course-accent, #18a058); } }
.lesson-navigation { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 0 0; > span { font-size: 12px; color: var(--course-muted, rgb(118, 124, 130)); } }
.lesson-sidebar { border-top: 1px solid var(--course-line, rgb(239, 239, 245)); border-bottom: 1px solid var(--course-line, rgb(239, 239, 245)); overflow: hidden; }
.outline-heading { padding: 12px 0; display: flex; align-items: center; justify-content: space-between; gap: 8px; h2 { margin: 0; font-size: 16px; font-weight: 500; } > span { font-size: 12px; color: var(--course-muted, rgb(118, 124, 130)); } }
.lesson-playlist { list-style: none; padding: 0 0 8px; margin: 0; max-height: 490px; overflow-y: auto; a { display: flex; align-items: center; gap: 12px; padding: 12px 8px; text-decoration: none; color: inherit; border-left: 2px solid transparent; } a:hover { background: var(--course-tint, rgba(24, 160, 88, .08)); } a.active { background: var(--course-tint, rgba(24, 160, 88, .08)); color: var(--course-accent, #18a058); border-color: var(--course-accent, #18a058); } }
.lesson-number { align-self: flex-start; font-size: 13px; font-variant-numeric: tabular-nums; padding-top: 2px; color: var(--course-muted, rgb(118, 124, 130)); }
.playlist-text { min-width: 0; flex: 1; display: flex; flex-direction: column; gap: 5px; }
.playlist-title { font-size: 14px; line-height: 1.5; overflow-wrap: anywhere; }
.playlist-status { color: var(--course-muted, rgb(118, 124, 130)); font-size: 12px; }
.active .playlist-title { font-weight: 500; }
.active .playlist-status, .active .lesson-number { color: var(--course-accent, #18a058); }
.playing-indicator { flex-shrink: 0; }
.outline-empty { margin: 16px 0; font-size: 13px; color: var(--course-muted, rgb(118, 124, 130)); }
.lesson-details { margin-top: 20px; scroll-margin-top: 16px; h2 { margin: 16px 0 10px; font-size: 16px; font-weight: 500; } }
.detail-tabs { display: flex; gap: 24px; border-bottom: 1px solid var(--course-line, rgb(239, 239, 245)); overflow-x: auto; button { flex-shrink: 0; padding: 12px 0; border: 0; border-bottom: 2px solid transparent; background: transparent; font: inherit; font-size: 14px; color: var(--course-muted, rgb(118, 124, 130)); cursor: pointer; } button[aria-selected="true"] { color: var(--course-accent, #18a058); border-color: var(--course-accent, #18a058); font-weight: 500; } }
.intro-layout { display: grid; grid-template-columns: minmax(0, 1fr) 240px; gap: 24px; padding-top: 4px; }
.lesson-copy, .teacher-panel { p { line-height: 1.7; font-size: 14px; white-space: pre-wrap; overflow-wrap: anywhere; max-width: 72ch; margin: 0 0 16px; } }
.teacher-panel { border-left: 1px solid var(--course-line, rgb(239, 239, 245)); padding-left: 20px; p { color: var(--course-muted, rgb(118, 124, 130)); } }
.teacher-link { display: flex; align-items: center; gap: 12px; color: inherit; text-decoration: none; margin-bottom: 12px; font-weight: 500; overflow-wrap: anywhere; .n-avatar { flex-shrink: 0; } }
.file-list { margin: 0; padding: 0; list-style: none; li { display: flex; align-items: center; gap: 16px; padding: 12px 0; border-bottom: 1px solid var(--course-line, rgb(239, 239, 245)); } li:last-child { border-bottom: 0; } .n-button { flex-shrink: 0; } }
.file-info { min-width: 0; flex: 1; display: flex; flex-direction: column; gap: 5px; strong { font-size: 14px; font-weight: 500; overflow-wrap: anywhere; } > span { font-size: 12px; color: var(--course-muted, rgb(118, 124, 130)); } }
.download-error { margin-top: 12px; }
.question-heading p, .empty-copy { font-size: 14px; color: var(--course-muted, rgb(118, 124, 130)); line-height: 1.7; }
.review-notice { margin: 16px 0; }
.question-list > :deep(.comment-item) { border-top: 1px solid var(--course-line, rgb(239, 239, 245)); }
.load-comments { display: flex; justify-content: center; padding: 16px 0 0; }
.page-state { display: flex; flex-direction: column; align-items: center; text-align: center; padding: 48px 16px; gap: 12px; h1 { font-size: 18px; font-weight: 500; } p { color: var(--course-muted, rgb(118, 124, 130)); } }
a:focus-visible, button:focus-visible { outline: 2px solid var(--course-accent, #18a058); outline-offset: 3px; }
@container course-watch (max-width: 780px) { .learning-layout, .intro-layout { grid-template-columns: minmax(0, 1fr); } .lesson-sidebar { margin-top: 4px; } .lesson-playlist { max-height: 280px; } .teacher-panel { border-left: 0; border-top: 1px solid var(--course-line, rgb(239, 239, 245)); padding: 4px 0 0; } .intro-layout { gap: 4px; } }
@media (max-width: 760px) { .watch-heading { align-items: flex-start; } .learning-layout, .intro-layout { grid-template-columns: minmax(0, 1fr); } .lesson-sidebar { margin-top: 4px; } .lesson-playlist { max-height: 280px; } .player-state { min-height: 190px; padding: 16px; p { font-size: 13px; } } .teacher-panel { border-left: 0; border-top: 1px solid var(--course-line, rgb(239, 239, 245)); padding: 4px 0 0; } .intro-layout { gap: 4px; } .lesson-navigation { gap: 8px; } }
@media (max-width: 420px) { .watch-heading { flex-direction: column; gap: 8px; } .detail-tabs { gap: 20px; } }
</style>
