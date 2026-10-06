<template>
  <section class="lesson-section" :aria-label="t('courseEditor.directory')">
    <div v-if="!canView" class="lesson-access">
      <p>{{ accessMessage }}</p>
      <n-button v-if="!storeUser.userLogined" secondary type="primary" @click="signIn">{{ t('course.action.login') }}</n-button>
      <router-link v-else-if="!storeUser.userInfo.contact_verified" :to="{ name: 'setting' }">{{ t('courseEditor.verifyContact') }}</router-link>
    </div>
    <template v-else>
      <div class="lesson-heading">
        <span>{{ t('courseEditor.lessonCount', { count: lessons.length }) }}</span>
        <n-button v-if="canManage" secondary type="primary" :disabled="busy" @click="editLesson()">{{ t('course.lesson.create') }}</n-button>
      </div>
      <p v-if="loading" class="lesson-status" role="status">{{ t('courseEditor.loading') }}</p>
      <div v-else-if="loadFailed" class="lesson-status" role="alert">
        <p>{{ t('courseEditor.loadFailed') }}</p>
        <n-button size="small" @click="loadLessons">{{ t('courseEditor.retry') }}</n-button>
      </div>
      <p v-else-if="!lessons.length" class="lesson-status">{{ canManage ? t('courseEditor.emptyManaged') : t('course.lesson.empty') }}</p>
      <ol v-else class="lesson-list">
        <li v-for="(lesson, index) in lessons" :key="lesson.id" class="lesson-row">
          <router-link class="lesson-link" :to="{ name: 'course', query: { id: course.id, lesson: lesson.id } }">
            <span class="lesson-number" aria-hidden="true">{{ String(index + 1).padStart(2, '0') }}</span>
            <span class="lesson-description">
              <span class="lesson-title">{{ lesson.title }}</span>
              <span class="lesson-meta">{{ lesson.attachments.some(isVideo) ? t('courseEditor.videoLesson') : t('courseEditor.readingLesson') }}</span>
            </span>
            <span class="lesson-watch">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 5 11 7-11 7z" fill="currentColor" /></svg>
              <span>{{ t('courseEditor.startLesson') }}</span>
            </span>
          </router-link>
          <div v-if="canManage" class="lesson-controls">
            <n-button size="small" :disabled="busy" :aria-label="t('courseEditor.editNamedLesson', { title: lesson.title })" @click="editLesson(lesson)">{{ t('common.edit') }}</n-button>
            <n-popconfirm :positive-text="t('common.delete')" :negative-text="t('common.cancel')" @positive-click="removeLesson(lesson)">
              <template #trigger><n-button size="small" quaternary type="error" :disabled="busy" :aria-label="t('courseEditor.deleteNamedLesson', { title: lesson.title })">{{ t('common.delete') }}</n-button></template>
              {{ t('course.lesson.deleteConfirm', { title: lesson.title }) }}
            </n-popconfirm>
          </div>
        </li>
      </ol>
    </template>

    <n-modal v-model:show="editor" preset="card" class="course-lesson-editor" :title="draft.id ? t('course.lesson.edit') : t('course.lesson.create')" style="width: min(720px, calc(100vw - 24px)); max-height: calc(100dvh - 24px); overflow-y: auto" :closable="!busy" :mask-closable="!busy" :close-on-esc="!busy">
      <n-form label-placement="top" @submit.prevent="saveLesson">
        <fieldset class="editor-section video-section">
          <legend>{{ t('courseEditor.lessonVideo') }}</legend>
          <p class="editor-help">{{ t('courseEditor.videoHelp', { limit: formatSize(uploadLimits.course_resource_max_bytes) }) }}</p>
          <div v-if="primaryVideo" class="primary-video-file">
            <n-form-item :label="t('courseEditor.videoName')">
              <n-input v-model:value="primaryVideo.name" :input-props="{ 'aria-label': t('courseEditor.videoName') }" :maxlength="255" :disabled="busy" />
            </n-form-item>
            <span class="file-size">{{ formatSize(primaryVideo.file_size) }}</span>
            <div class="editor-file-actions">
              <n-button v-if="canUpload" size="small" :loading="uploading && uploadTarget === 'video'" :disabled="busy" @click="chooseFiles('video')">{{ t('courseEditor.replaceVideo') }}</n-button>
              <n-button size="small" quaternary :disabled="busy" @click="removeFile(primaryVideo)">{{ t('courseEditor.removeVideo') }}</n-button>
            </div>
          </div>
          <div v-else class="video-empty">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 3h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2Zm4 4v10l8-5Z" fill="currentColor" /></svg>
            <p>{{ t('courseEditor.noVideo') }}</p>
            <n-button v-if="canUpload" type="primary" :loading="uploading && uploadTarget === 'video'" :disabled="busy" @click="chooseFiles('video')">{{ t('courseEditor.uploadVideo') }}</n-button>
          </div>
          <div v-if="uploading && uploadTarget === 'video'" class="upload-progress" role="status" aria-live="polite">
            <p>{{ t('courseEditor.uploading', { name: uploadingName }) }}</p>
            <n-progress type="line" :percentage="uploadProgress" />
            <n-button size="small" @click="controller?.abort()">{{ t('courseEditor.cancelUpload') }}</n-button>
          </div>
        </fieldset>

        <n-form-item :label="t('course.lesson.name')" required>
          <n-input v-model:value="draft.title" :input-props="{ 'aria-label': t('course.lesson.name') }" :placeholder="t('courseEditor.titlePlaceholder')" :maxlength="128" :disabled="busy" />
        </n-form-item>
        <n-form-item :label="t('course.lesson.intro')">
          <n-input v-model:value="draft.intro" :input-props="{ 'aria-label': t('course.lesson.intro') }" :placeholder="t('courseEditor.introPlaceholder')" type="textarea" :autosize="{ minRows: 4, maxRows: 10 }" :maxlength="2000" :disabled="busy" />
        </n-form-item>

        <fieldset class="editor-section materials-section">
          <legend>{{ t('courseEditor.materials') }}</legend>
          <p class="editor-help">{{ t('courseEditor.materialsHelp') }}</p>
          <p v-if="!materials.length" class="empty-materials">{{ t('courseEditor.noMaterials') }}</p>
          <div v-for="file in materials" :key="file.attachment_id" class="material-row">
            <div class="material-name">
              <n-input v-model:value="file.name" :input-props="{ 'aria-label': t('course.lesson.fileName') }" :maxlength="255" :disabled="busy" />
              <span class="file-size">{{ formatSize(file.file_size) }}</span>
            </div>
            <div class="editor-file-actions">
              <n-button v-if="isVideo(file)" size="small" :disabled="busy" @click="setPrimaryVideo(file)">{{ t('courseEditor.useAsVideo') }}</n-button>
              <n-button size="small" quaternary :disabled="busy" :aria-label="t('courseEditor.removeNamedFile', { name: file.name })" @click="removeFile(file)">{{ t('courseEditor.removeFile') }}</n-button>
            </div>
          </div>
          <div v-if="canUpload" class="material-upload-actions">
            <n-button :disabled="busy" @click="chooseFiles('attachment')">{{ t('courseEditor.uploadMaterial', { limit: formatSize(uploadLimits.course_attachment_max_bytes) }) }}</n-button>
            <n-button quaternary :disabled="busy" @click="chooseFiles('resource')">{{ t('courseEditor.uploadResource', { limit: formatSize(uploadLimits.course_resource_max_bytes) }) }}</n-button>
          </div>
          <p v-else class="editor-help">{{ t('courseEditor.uploadPermission') }}</p>
        </fieldset>
        <input ref="videoPicker" type="file" accept="video/*" hidden @change="uploadFiles($event, 'video')" />
        <input ref="materialPicker" type="file" multiple hidden @change="uploadFiles($event, materialKind)" />
        <div v-if="uploading && uploadTarget !== 'video'" class="upload-progress" role="status" aria-live="polite">
          <p>{{ t('courseEditor.uploading', { name: uploadingName }) }}</p>
          <n-progress type="line" :percentage="uploadProgress" />
          <n-button size="small" @click="controller?.abort()">{{ t('courseEditor.cancelUpload') }}</n-button>
        </div>
        <details class="lesson-order">
          <summary>{{ t('courseEditor.order') }}</summary>
          <n-form-item :label="t('courseEditor.orderHelp')">
            <n-input-number v-model:value="draft.sort" :min="0" :precision="0" :disabled="busy" :input-props="{ 'aria-label': t('course.list.sortOrder') }" />
          </n-form-item>
        </details>
        <n-alert v-if="error" type="error" class="editor-error">{{ error }}</n-alert>
        <div class="editor-footer">
          <p>{{ t('courseEditor.publishHelp') }}</p>
          <div class="editor-file-actions">
            <n-button :disabled="busy" @click="editor = false">{{ t('common.cancel') }}</n-button>
            <n-button attr-type="submit" type="primary" :loading="saving" :disabled="uploading || deleting">{{ t('courseEditor.saveLesson') }}</n-button>
          </div>
        </div>
      </n-form>
    </n-modal>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreUser } from '@/store/user';
import { useStoreMain } from '@/store/main';
import { useStoreProfile } from '@/store/profile';
import { getCourseLessons, createCourseLesson, updateCourseLesson, deleteCourseLesson, uploadCourseFile, type CourseItem, type CourseLesson, type UploadedCourseFile } from '@/api/course';

const props = withDefaults(defineProps<{ course: CourseItem; managing?: boolean }>(), { managing: false });
const emit = defineEmits<{ changed: [] }>();
const { t } = useI18n();
const storeUser = useStoreUser();
const storeMain = useStoreMain();
const storeProfile = useStoreProfile();
const uploadLimits = computed(() => storeProfile.profile.uploadLimits);
const canView = computed(() => storeUser.hasPermission('course.view'));
const canManage = computed(() => props.managing && (storeUser.hasPermission('course.manage') || (storeUser.hasPermission('course.manage_own') && props.course.teacher_id === storeUser.userInfo.id)));
const canUpload = computed(() => canManage.value && storeUser.hasPermission('course.upload'));
const accessMessage = computed(() => !storeUser.userLogined ? t('courseEditor.signInHelp') : !storeUser.userInfo.contact_verified ? t('courseEditor.verifyHelp') : t('courseEditor.accessHelp'));
function signIn() { storeMain.triggerAuthKey('signin'); storeMain.triggerAuth(true); }

const lessons = ref<CourseLesson[]>([]);
const loading = ref(false);
const loadFailed = ref(false);
let loadRevision = 0;
async function loadLessons() {
  const revision = ++loadRevision;
  if (!canView.value) { lessons.value = []; loading.value = false; return; }
  loading.value = true; loadFailed.value = false;
  try {
    const result = await getCourseLessons(props.course.id);
    if (revision === loadRevision) lessons.value = result.lessons || [];
  } catch { if (revision === loadRevision) loadFailed.value = true; }
  finally { if (revision === loadRevision) loading.value = false; }
}
watch([() => props.course.id, canView], () => { lessons.value = []; void loadLessons(); }, { immediate: true });

const formatSize = (size: number) => size >= 1024 ** 3 ? `${(size / 1024 ** 3).toFixed(1)} GB` : size >= 1024 ** 2 ? `${(size / 1024 ** 2).toFixed(1)} MB` : size >= 1024 ? `${(size / 1024).toFixed(1)} KB` : `${size} B`;
const isVideo = (file: UploadedCourseFile) => file.mime_type.startsWith('video/');
const editor = ref(false);
const saving = ref(false);
const uploading = ref(false);
const deleting = ref(false);
const busy = computed(() => saving.value || uploading.value || deleting.value);
const error = ref('');
const draft = reactive({ id: 0, title: '', intro: '', sort: 0, attachments: [] as UploadedCourseFile[] });
const primaryVideo = computed(() => draft.attachments.find(isVideo));
const materials = computed(() => draft.attachments.filter((file) => file !== primaryVideo.value));
function editLesson(lesson?: CourseLesson) {
  if (!canManage.value || busy.value) return;
  Object.assign(draft, { id: lesson?.id || 0, title: lesson?.title || '', intro: lesson?.intro || '', sort: lesson?.sort ?? (lessons.value.length ? Math.max(...lessons.value.map((item) => item.sort)) + 1 : 0), attachments: (lesson?.attachments || []).map((file) => ({ ...file })) });
  error.value = ''; editor.value = true;
}
async function saveLesson() {
  if (!canManage.value || busy.value) return;
  if (!draft.title.trim() || draft.attachments.some((file) => !file.name.trim())) { error.value = t('course.lesson.inputRequired'); return; }
  saving.value = true; error.value = '';
  try {
    const data = { course_id: props.course.id, title: draft.title.trim(), intro: draft.intro, sort: draft.sort || 0, attachments: draft.attachments.map((file) => ({ attachment_id: file.attachment_id, name: file.name.trim(), kind: file.kind })) };
    if (draft.id) await updateCourseLesson({ ...data, id: draft.id }); else await createCourseLesson(data);
    editor.value = false; await loadLessons(); emit('changed'); window.$message.success(t('common.operationSuccess'));
  } catch { error.value = t('course.lesson.saveFailed'); }
  finally { saving.value = false; }
}
async function removeLesson(lesson: CourseLesson) {
  if (!canManage.value || busy.value) return false;
  deleting.value = true;
  try { await deleteCourseLesson(lesson.id); await loadLessons(); emit('changed'); window.$message.success(t('course.deleteSuccess')); }
  catch { return false; }
  finally { deleting.value = false; }
}
function removeFile(file: UploadedCourseFile) {
  if (busy.value) return;
  const index = draft.attachments.indexOf(file);
  if (index >= 0) draft.attachments.splice(index, 1);
}
function setPrimaryVideo(file: UploadedCourseFile) {
  if (busy.value || !isVideo(file)) return;
  removeFile(file); draft.attachments.unshift(file);
}

type UploadTarget = 'video' | 'attachment' | 'resource';
const videoPicker = ref<HTMLInputElement>();
const materialPicker = ref<HTMLInputElement>();
const materialKind = ref<'attachment' | 'resource'>('attachment');
const uploadTarget = ref<UploadTarget>('video');
const uploadingName = ref('');
const uploadProgress = ref(0);
let controller: AbortController | undefined;
function chooseFiles(target: UploadTarget) {
  if (!canUpload.value || busy.value) return;
  if (target === 'video') videoPicker.value?.click();
  else { materialKind.value = target; materialPicker.value?.click(); }
}
async function uploadFiles(event: Event, target: UploadTarget) {
  const input = event.target as HTMLInputElement;
  const files = Array.from(input.files || []); input.value = '';
  if (!files.length || !canUpload.value || busy.value) return;
  const kind = target === 'attachment' ? 'attachment' : 'resource';
  const limit = kind === 'resource' ? uploadLimits.value.course_resource_max_bytes : uploadLimits.value.course_attachment_max_bytes;
  if (files.some((file) => file.size === 0 || file.size > limit)) { error.value = t('course.lesson.sizeError', { limit: formatSize(limit) }); return; }
  if (target === 'video' && (files.length !== 1 || !files[0].type.startsWith('video/'))) { error.value = t('courseEditor.videoOnly'); return; }
  uploadTarget.value = target; uploading.value = true; error.value = '';
  const uploadController = new AbortController(); controller = uploadController;
  try {
    for (const file of files) {
      uploadingName.value = file.name; uploadProgress.value = 0;
      const uploaded = await uploadCourseFile(file, kind, uploadController.signal, (progress) => { uploadProgress.value = progress; });
      if (uploadController.signal.aborted) break;
      if (target === 'video') {
        if (!isVideo(uploaded)) throw new Error(t('courseEditor.videoOnly'));
        const previous = draft.attachments.findIndex(isVideo);
        if (previous >= 0) draft.attachments.splice(previous, 1, uploaded);
        else draft.attachments.unshift(uploaded);
      } else draft.attachments.push(uploaded);
    }
  } catch (failure) { if (!uploadController.signal.aborted) error.value = failure instanceof Error ? failure.message : t('course.upload.failed'); }
  finally { uploading.value = false; controller = undefined; }
}
watch([() => props.course.id, canManage], () => { controller?.abort(); editor.value = false; });
onBeforeUnmount(() => { loadRevision++; controller?.abort(); });
</script>

<style scoped lang="less">
.lesson-section { color: var(--course-text, rgb(31, 34, 37)); }
.lesson-heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 0; color: var(--course-muted, rgb(118, 124, 130)); font-size: 13px; }
.lesson-list { margin: 0; padding: 0; list-style: none; }
.lesson-row { display: flex; align-items: center; border-top: 1px solid var(--course-line, rgb(239, 239, 245)); }
.lesson-link { display: flex; flex: 1; align-items: center; gap: 16px; min-width: 0; padding: 16px 8px 16px 0; color: inherit; text-decoration: none; border-radius: 3px; }
.lesson-link:hover { background: var(--course-tint, rgba(24, 160, 88, .08)); color: var(--course-accent, #18a058); }
.lesson-link:focus-visible, .lesson-access a:focus-visible { outline: 2px solid var(--course-accent, #18a058); outline-offset: 3px; }
.lesson-number { flex: 0 0 28px; font-variant-numeric: tabular-nums; color: var(--course-muted, rgb(118, 124, 130)); font-size: 14px; text-align: center; }
.lesson-description { display: grid; gap: 4px; min-width: 0; }
.lesson-title { font-size: 14px; font-weight: 500; line-height: 1.55; overflow-wrap: anywhere; }
.lesson-meta { color: var(--course-muted, rgb(118, 124, 130)); font-size: 12px; }
.lesson-watch { display: inline-flex; align-items: center; gap: 5px; flex-shrink: 0; margin-left: auto; color: var(--course-accent, #18a058); font-size: 13px; }
.lesson-watch svg { width: 18px; height: 18px; }
.lesson-controls { display: flex; gap: 5px; padding-left: 16px; }
.lesson-status, .lesson-access { margin: 0; padding: 16px 0; color: var(--course-muted, rgb(118, 124, 130)); line-height: 1.7; }
.lesson-access p { margin: 0 0 12px; }
.lesson-access a { color: var(--course-accent, #18a058); }
.editor-section { min-width: 0; margin: 0 0 16px; padding: 16px; border: 1px solid var(--n-border-color); border-radius: 3px; }
.editor-section legend { padding: 0 6px; font-size: 14px; font-weight: 500; }
.editor-help { margin: 0 0 14px; opacity: .75; font-size: 13px; line-height: 1.65; }
.video-section { background: transparent; }
.video-empty { text-align: center; padding: 8px 0 12px; }
.video-empty svg { width: 30px; height: 30px; opacity: .55; }
.video-empty p { margin: 8px 0 16px; opacity: .75; font-size: 13px; }
.primary-video-file .n-form-item { margin-bottom: -6px; }
.primary-video-file > .editor-file-actions { margin-top: 12px; }
.file-size { font-size: 12px; opacity: .65; white-space: nowrap; }
.material-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; padding: 12px 0; border-top: 1px solid var(--n-border-color); }
.material-name { flex: 1; min-width: 0; display: grid; gap: 4px; }
.editor-file-actions, .material-upload-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.material-upload-actions { margin-top: 16px; }
.empty-materials { margin: 12px 0; opacity: .6; font-size: 13px; }
.upload-progress { margin: 0 0 16px; padding: 16px; border: 1px solid var(--n-border-color); border-radius: 3px; }
.upload-progress p { margin: 0 0 8px; overflow-wrap: anywhere; }
.upload-progress .n-button { margin-top: 8px; }
.lesson-order { margin: 0 0 24px; }
.lesson-order summary { cursor: pointer; font-size: 13px; opacity: .8; padding: 4px 0; }
.lesson-order .n-form-item { margin-top: 12px; }
.editor-error { margin-bottom: 16px; }
.editor-footer { display: flex; align-items: center; justify-content: space-between; gap: 20px; border-top: 1px solid var(--n-border-color); padding-top: 16px; }
.editor-footer p { max-width: 280px; margin: 0; opacity: .65; line-height: 1.6; font-size: 12px; }
@media (max-width: 640px) {
  .lesson-row { flex-wrap: wrap; }
  .lesson-link { gap: 10px; padding: 16px 0; }
  .lesson-controls { width: 100%; justify-content: flex-end; padding: 0 0 12px; }
  .lesson-watch > span { display: none; }
  .material-row { flex-direction: column; }
  .material-name { width: 100%; }
  .editor-footer { flex-direction: column; align-items: stretch; }
  .editor-footer > .editor-file-actions { justify-content: flex-end; }
}
</style>
