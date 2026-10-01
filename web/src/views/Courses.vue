<template>
    <div>
        <main-nav :title="title" />

        <div class="courses-wrap">
            <!-- 搜索与管理工具栏 -->
            <div class="toolbar">
                <n-input
                    v-model:value="keyword"
                    class="search-input"
                    :placeholder="t('course.list.searchPlaceholder')"
                    clearable
                    @keyup.enter="doSearch"
                    @clear="clearSearch"
                >
                    <template #prefix>
                        <n-icon><search-outline /></n-icon>
                    </template>
                </n-input>
                <n-button type="primary" secondary round @click="doSearch">{{ t('common.search') }}</n-button>
                <template v-if="userInfo.is_admin">
                    <n-button secondary round @click="openGroupModal()">{{ t('course.list.createGroup') }}</n-button>
                    <n-button secondary round type="info" @click="openCourseModal()">{{ t('course.list.createCourse') }}</n-button>
                </template>
            </div>

            <!-- 搜索结果(扁平分页) -->
            <template v-if="searching">
                <div class="section-header">
                    <span class="section-title">{{ t('course.list.searchResultTitle', { keyword: searchKeyword }) }}</span>
                    <n-button text size="small" @click="clearSearch">{{ t('course.list.backToGroups') }}</n-button>
                </div>
                <div class="course-grid">
                    <course-card
                        v-for="course in searchList"
                        :key="course.id"
                        :course="course"
                        :is-admin="userInfo.is_admin"
                        @edit="openCourseModal(course)"
                        @delete="execDeleteCourse(course)"
                    />
                </div>
                <n-empty v-if="!searchLoading && searchList.length === 0" :description="t('course.list.noSearchResult')" />
                <InfiniteLoading :key="'search-' + searchLoadKey" @infinite="onSearchInfinite">
                    <template #complete><span class="load-end">{{ t('course.list.bottomLine') }}</span></template>
                </InfiniteLoading>
            </template>

            <!-- 分组浏览 -->
            <template v-else>
                <div v-if="groups.length === 0 && !loadingGroups" class="empty-wrap">
                    <n-empty size="large" :description="t('course.list.noGroups')" />
                </div>
                <div v-for="group in groups" :key="group.id" class="group-section">
                    <div class="section-header">
                        <span class="section-title">
                            {{ group.name }}
                            <span class="section-count">{{ t('course.list.courseCount', { count: group.course_count }) }}</span>
                        </span>
                        <div class="section-ops">
                            <n-button
                                v-if="group.course_count > groupPreviewSize"
                                text
                                size="small"
                                type="primary"
                                @click="enterGroup(group.id)"
                            >
                                {{ t('course.list.viewAll') }}
                            </n-button>
                            <template v-if="userInfo.is_admin">
                                <n-button text size="small" @click="openGroupModal(group)">{{ t('common.edit') }}</n-button>
                                <n-popconfirm
                                    :negative-text="t('common.cancel')"
                                    :positive-text="t('common.delete')"
                                    @positive-click="execDeleteGroup(group)"
                                >
                                    <template #trigger>
                                        <n-button text size="small" type="error">{{ t('common.delete') }}</n-button>
                                    </template>
                                    {{ t('course.list.deleteGroupConfirm', { name: group.name }) }}
                                </n-popconfirm>
                            </template>
                        </div>
                    </div>
                    <div class="course-grid">
                        <course-card
                            v-for="course in groupCourses[group.id] || []"
                            :key="course.id"
                            :course="course"
                            :is-admin="userInfo.is_admin"
                            @edit="openCourseModal(course)"
                            @delete="execDeleteCourse(course)"
                        />
                        <div v-if="(groupCourses[group.id] || []).length === 0" class="group-empty">
                            {{ t('course.list.groupEmpty') }}
                        </div>
                    </div>
                </div>
            </template>
        </div>

        <!-- 分组查看全部抽屉 -->
        <n-drawer v-model:show="groupDrawerShow" :width="520" placement="right">
            <n-drawer-content :title="drawerGroupName" closable>
                <div class="drawer-list">
                    <course-card
                        v-for="course in drawerList"
                        :key="course.id"
                        :course="course"
                        :is-admin="userInfo.is_admin"
                        @edit="openCourseModal(course)"
                        @delete="execDeleteCourse(course)"
                    />
                </div>
                <n-empty v-if="!drawerLoading && drawerList.length === 0" :description="t('course.list.groupEmpty')" />
                <InfiniteLoading :key="'group-' + drawerLoadKey" @infinite="onDrawerInfinite">
                    <template #complete><span class="load-end">{{ t('course.list.bottomLine') }}</span></template>
                </InfiniteLoading>
            </n-drawer-content>
        </n-drawer>

        <!-- 分组编辑弹窗 -->
        <n-modal v-model:show="groupModalShow" preset="card" :title="groupForm.id > 0 ? t('course.list.editGroup') : t('course.list.createGroup')" style="width: 420px">
            <n-form label-placement="left" label-width="80">
                <n-form-item :label="t('course.list.groupName')" required>
                    <n-input v-model:value="groupForm.name" maxlength="64" show-count :placeholder="t('course.list.groupNamePlaceholder')" />
                </n-form-item>
                <n-form-item :label="t('course.list.sortOrder')">
                    <n-input-number v-model:value="groupForm.sort" :min="0" :placeholder="t('course.list.sortHint')" />
                </n-form-item>
            </n-form>
            <template #footer>
                <n-button type="primary" :loading="groupSaving" @click="saveGroup">{{ t('common.save') }}</n-button>
            </template>
        </n-modal>

        <!-- 课程编辑弹窗 -->
        <n-modal v-model:show="courseModalShow" preset="card" :title="courseForm.id > 0 ? t('course.list.editCourse') : t('course.list.createCourse')" style="width: 640px">
            <n-form label-placement="left" label-width="80">
                <n-form-item :label="t('course.list.belongGroup')" required>
                    <n-select
                        v-model:value="courseForm.group_id"
                        :options="groupOptions"
                        :placeholder="t('course.list.selectGroup')"
                    />
                </n-form-item>
                <n-form-item :label="t('course.list.courseTitle')" required>
                    <n-input v-model:value="courseForm.title" maxlength="128" show-count :placeholder="t('course.list.courseTitlePlaceholder')" />
                </n-form-item>
                <n-form-item :label="t('course.list.courseIntro')">
                    <n-input
                        v-model:value="courseForm.intro"
                        type="textarea"
                        maxlength="2000"
                        show-count
                        :autosize="{ minRows: 3, maxRows: 8 }"
                        :placeholder="t('course.list.courseIntroPlaceholder')"
                    />
                </n-form-item>
                <n-form-item :label="t('course.list.courseTeacher')" required>
                    <n-select
                        v-model:value="courseForm.teacher_id"
                        filterable
                        remote
                        :options="teacherOptions"
                        :loading="teacherLoading"
                        :placeholder="t('course.list.teacherSearchPlaceholder')"
                        @search="searchTeachers"
                        @focus="searchTeachers('')"
                    />
                </n-form-item>
                <n-form-item :label="t('course.list.lessons')" required>
                    <div class="lesson-editor">
                      <div v-for="(lesson, index) in lessons" :key="lesson.clientKey" class="lesson-card">
                        <div class="lesson-head">
                          <strong>{{ t('course.list.lessonNumber', { number: index + 1 }) }}</strong>
                          <n-button v-if="lessons.length > 1" text type="error" :disabled="courseUploadsPending" @click="removeLesson(index)">{{ t('common.delete') }}</n-button>
                        </div>
                        <n-input v-model:value="lesson.title" maxlength="128" :placeholder="t('course.list.lessonTitle')" />
                        <n-input v-model:value="lesson.summary" type="textarea" maxlength="2000" :placeholder="t('course.list.lessonSummary')" />
                        <n-upload
                            :show-file-list="false"
                            :custom-request="noopUpload"
                            @before-upload="(data) => beforeVideoPick(data, lesson.clientKey)"
                        >
                            <n-button secondary>
                                {{ lesson.videoName || (lesson.video ? t('course.list.videoReadyUploaded') : t('course.list.selectOptionalVideo')) }}
                            </n-button>
                        </n-upload>
                        <n-upload :show-file-list="false" :custom-request="noopUpload" @before-upload="(data) => beforeAttachmentPick(data, lesson.clientKey)">
                          <n-button secondary>{{ t('course.list.addAttachment') }}</n-button>
                        </n-upload>
                        <div v-for="(attachment, attachmentIndex) in lesson.attachments" :key="attachment.url" class="lesson-attachment">
                          <span>{{ attachment.name }}</span><n-button text type="error" @click="lesson.attachments.splice(attachmentIndex, 1)">×</n-button>
                        </div>
                      </div>
                      <n-button dashed block :disabled="courseUploadsPending" @click="addLesson">{{ t('course.list.addLesson') }}</n-button>
                    </div>
                </n-form-item>
                <n-form-item :label="t('course.list.courseCover')">
                    <div class="cover-wrap">
                        <img v-if="coverPreview" :src="coverPreview" class="cover-preview" :alt="t('course.list.coverPreview')" />
                        <span class="cover-hint">{{ t('course.list.coverHint') }}</span>
                    </div>
                </n-form-item>
            </n-form>
            <template #footer>
                <n-button type="primary" :loading="courseSaving" :disabled="courseUploadsPending" @click="saveCourse">{{ t('common.save') }}</n-button>
            </template>
        </n-modal>
    </div>
</template>

<script setup lang="ts">
import {
  type CourseGroup,
  type CourseItem,
  type CourseLesson,
  createCourse,
  createCourseGroup,
  deleteCourse,
  deleteCourseGroup,
  getCourse,
  getCourseGroups,
  getCourseList,
  getCourseUploadCredential,
  updateCourse,
  updateCourseGroup,
} from '@/api/course';
import CourseCard from '@/components/course/course-card.vue';
import { useStoreUser } from '@/store/user';
import { TOKEN_KEY } from '@/store/user';
import { Api } from '@/utils/request';
import { SearchOutline } from '@vicons/ionicons5';
import axios from 'axios';
import type { UploadCustomRequestOptions } from 'naive-ui';
import { storeToRefs } from 'pinia';
import InfiniteLoading from 'v3-infinite-loading';
import { computed, onMounted, reactive, ref } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();
const title = computed(() => t('course.list.title'));
const storeUser = useStoreUser();
const { userInfo } = storeToRefs(storeUser);

// ===== 分组浏览 =====
const groups = ref<CourseGroup[]>([]);
const groupCourses = ref<Record<number, CourseItem[]>>({});
const loadingGroups = ref(false);
const groupPreviewSize = 8;

const loadGroups = async () => {
  loadingGroups.value = true;
  try {
    const res = await getCourseGroups();
    groups.value = res.groups || [];
    // 每个分组加载前N门课程预览
    for (const g of groups.value) {
      loadGroupCourses(g.id);
    }
  } catch (_err) {
    // do nothing
  } finally {
    loadingGroups.value = false;
  }
};

const loadGroupCourses = async (groupId: number) => {
  try {
    const res = await getCourseList({
      group_id: groupId,
      page: 1,
      page_size: groupPreviewSize,
    });
    groupCourses.value[groupId] = res.list || [];
  } catch (_err) {
    // do nothing
  }
};

// ===== 搜索 =====
const keyword = ref('');
const searching = ref(false);
const searchKeyword = ref('');
const searchList = ref<CourseItem[]>([]);
const searchPage = ref(1);
const searchTotal = ref(0);
const searchLoading = ref(false);
// 每次搜索自增, 强制InfiniteLoading重挂载(重置其loaded/complete内部状态)
const searchLoadKey = ref(0);
const pageSize = 20;

const doSearch = () => {
  const k = keyword.value.trim();
  if (!k) {
    clearSearch();
    return;
  }
  searching.value = true;
  searchKeyword.value = k;
  searchList.value = [];
  searchPage.value = 1;
  searchTotal.value = 0;
  searchLoadKey.value++;
};
const clearSearch = () => {
  searching.value = false;
  keyword.value = '';
  searchList.value = [];
};
// 返回是否还有更多(供InfiniteLoading决定loaded/complete)
const loadSearchData = async (): Promise<boolean> => {
  if (searchLoading.value) return false;
  if (searchTotal.value > 0 && searchList.value.length >= searchTotal.value)
    return false;
  searchLoading.value = true;
  try {
    const res = await getCourseList({
      keyword: searchKeyword.value,
      page: searchPage.value,
      page_size: pageSize,
    });
    searchList.value = searchList.value.concat(res.list || []);
    searchTotal.value = res.pager?.total_rows || 0;
    searchPage.value++;
    return searchList.value.length < searchTotal.value;
  } catch (_err) {
    throw new Error('load failed');
  } finally {
    searchLoading.value = false;
  }
};
// InfiniteLoading 收尾: 必须调用 $state.loaded()/complete() 否则spinner不消失
const onSearchInfinite = async ($state: any) => {
  try {
    const hasMore = await loadSearchData();
    if (hasMore) {
      $state.loaded();
    } else {
      $state.complete();
    }
  } catch (_err) {
    $state.error();
  }
};

// ===== 分组查看全部(抽屉内分页) =====
const groupDrawerShow = ref(false);
const drawerGroupId = ref(0);
const drawerGroupName = ref('');
const drawerList = ref<CourseItem[]>([]);
const drawerPage = ref(1);
const drawerTotal = ref(0);
const drawerLoading = ref(false);
const drawerLoadKey = ref(0);

const enterGroup = (groupId: number) => {
  const g = groups.value.find((i) => i.id === groupId);
  drawerGroupId.value = groupId;
  drawerGroupName.value = g?.name || '';
  drawerList.value = [];
  drawerPage.value = 1;
  drawerTotal.value = 0;
  groupDrawerShow.value = true;
  drawerLoadKey.value++;
};
const loadDrawerData = async (): Promise<boolean> => {
  if (drawerLoading.value) return false;
  if (drawerTotal.value > 0 && drawerList.value.length >= drawerTotal.value)
    return false;
  drawerLoading.value = true;
  try {
    const res = await getCourseList({
      group_id: drawerGroupId.value,
      page: drawerPage.value,
      page_size: pageSize,
    });
    drawerList.value = drawerList.value.concat(res.list || []);
    drawerTotal.value = res.pager?.total_rows || 0;
    drawerPage.value++;
    return drawerList.value.length < drawerTotal.value;
  } catch (_err) {
    throw new Error('load failed');
  } finally {
    drawerLoading.value = false;
  }
};
const onDrawerInfinite = async ($state: any) => {
  try {
    const hasMore = await loadDrawerData();
    if (hasMore) {
      $state.loaded();
    } else {
      $state.complete();
    }
  } catch (_err) {
    $state.error();
  }
};

// ===== 分组管理 =====
const groupModalShow = ref(false);
const groupSaving = ref(false);
const groupForm = reactive({ id: 0, name: '', sort: 0 });

const openGroupModal = (group?: CourseGroup) => {
  groupForm.id = group?.id || 0;
  groupForm.name = group?.name || '';
  groupForm.sort = group?.sort || 0;
  groupModalShow.value = true;
};
const saveGroup = async () => {
  if (!groupForm.name.trim()) {
    window.$message.warning(t('course.list.inputGroupName'));
    return;
  }
  groupSaving.value = true;
  try {
    if (groupForm.id > 0) {
      await updateCourseGroup({
        id: groupForm.id,
        name: groupForm.name.trim(),
        sort: groupForm.sort,
      });
    } else {
      await createCourseGroup({
        name: groupForm.name.trim(),
        sort: groupForm.sort,
      });
    }
    window.$message.success(t('course.list.saveSuccess'));
    groupModalShow.value = false;
    loadGroups();
  } catch (_err) {
    // 错误提示已由拦截器弹出
  } finally {
    groupSaving.value = false;
  }
};
const execDeleteGroup = async (group: CourseGroup) => {
  try {
    await deleteCourseGroup({ id: group.id });
    window.$message.success(t('course.deleteSuccess'));
    loadGroups();
  } catch (_err) {
    // do nothing
  }
};

// ===== 课程管理 =====
const courseModalShow = ref(false);
const courseSaving = ref(false);
const courseForm = reactive({
  id: 0,
  group_id: null as number | null,
  teacher_id: null as number | null,
  title: '',
  intro: '',
});
type EditableLesson = CourseLesson & {
  videoName?: string;
  clientKey: string;
  uploadsPending: number;
};
const lessons = ref<EditableLesson[]>([]);
const attachmentUploading = ref(false);
let lessonKeySequence = 0;
const nextLessonKey = () => `lesson-${Date.now()}-${lessonKeySequence++}`;
const newLesson = (title = ''): EditableLesson => ({
  title,
  summary: '',
  video: '',
  sort: 0,
  attachments: [],
  clientKey: nextLessonKey(),
  uploadsPending: 0,
});
const courseUploadsPending = computed(() =>
  lessons.value.some((lesson) => lesson.uploadsPending > 0),
);
const findLesson = (clientKey: string) =>
  lessons.value.find((lesson) => lesson.clientKey === clientKey);
const addLesson = () => lessons.value.push(newLesson());
const removeLesson = (index: number) => lessons.value.splice(index, 1);

const groupOptions = computed(() =>
  groups.value.map((g) => ({ label: g.name, value: g.id })),
);

// 老师远程搜索(复用管理员用户搜索)
const teacherOptions = ref<{ label: string; value: number }[]>([]);
const teacherLoading = ref(false);
const searchTeachers = async (k: string) => {
  teacherLoading.value = true;
  try {
    const res = await Api.v1.admin.get.user.list({
      keyword: k,
      page: 1,
      page_size: 20,
    });
    teacherOptions.value = (res.list || []).map((u: any) => ({
      label: `${u.nickname} (@${u.username})`,
      value: u.id,
    }));
  } catch (_err) {
    // do nothing
  } finally {
    teacherLoading.value = false;
  }
};

// 视频上传(直传优先, 代理回退)
const videoUploading = ref(false);
const videoProgress = ref(0);
// 封面(canvas截帧)
const coverBlob = ref<Blob | null>(null);
const coverPreview = ref('');
const coverUrl = ref('');

const noopUpload = (_options: UploadCustomRequestOptions) => {
  // 仅用于选择文件, 实际上传走 beforeVideoPick 自定义流程
};

const beforeVideoPick = async (data: any, lessonKey: string) => {
  const file: File | undefined = data.file?.file;
  if (!file) return false;
  const ext = '.' + (file.name.split('.').pop() || '').toLowerCase();
  if (!['.mp4', '.mov'].includes(ext)) {
    window.$message.warning(t('course.list.videoFormatError'));
    return false;
  }
  if (file.size > 1024 * 1024 * 500) {
    window.$message.warning(t('course.list.videoSizeError'));
    return false;
  }
  const lesson = findLesson(lessonKey);
  if (!lesson) return false;
  lesson.videoName = file.name;
  // 用本地文件截帧生成封面(避免OSS跨域污染canvas)
  captureCover(file);
  await uploadVideo(file, ext, lessonKey);
  return false;
};

// canvas截取视频约1秒处画面作为封面
const captureCover = (file: File) => {
  const url = URL.createObjectURL(file);
  const video = document.createElement('video');
  video.muted = true;
  video.playsInline = true;
  video.preload = 'auto';
  video.src = url;
  const cleanup = () => URL.revokeObjectURL(url);
  video.addEventListener('loadeddata', () => {
    video.currentTime = Math.min(1, (video.duration || 2) / 2);
  });
  video.addEventListener('seeked', () => {
    try {
      const canvas = document.createElement('canvas');
      canvas.width = video.videoWidth || 640;
      canvas.height = video.videoHeight || 360;
      canvas
        .getContext('2d')
        ?.drawImage(video, 0, 0, canvas.width, canvas.height);
      canvas.toBlob(
        (b) => {
          cleanup();
          if (!b) return;
          coverBlob.value = b;
          if (coverPreview.value) URL.revokeObjectURL(coverPreview.value);
          coverPreview.value = URL.createObjectURL(b);
          // 封面即时上传到图床(复用公开图片通道)
          uploadCover(b);
        },
        'image/jpeg',
        0.85,
      );
    } catch (_err) {
      cleanup();
    }
  });
  video.addEventListener('error', cleanup);
};

// 封面走现有附件图片上传
const uploadCover = async (blob: Blob) => {
  try {
    const form = new FormData();
    form.append('type', 'public/image');
    form.append('file', new File([blob], 'cover.jpg', { type: 'image/jpeg' }));
    const res = await axios.post(
      import.meta.env.VITE_HOST + '/v1/attachment',
      form,
      {
        headers: { Authorization: 'Bearer ' + localStorage.getItem(TOKEN_KEY) },
      },
    );
    if (res.data?.code === 0) {
      coverUrl.value = res.data.data.content;
    }
  } catch (_err) {
    window.$message.warning(t('course.list.coverGenFailed'));
  }
};

// 视频上传: 先取凭证, direct=浏览器直传AliOSS / proxy=后端中转
const uploadVideo = async (file: File, ext: string, lessonKey: string) => {
  const lesson = findLesson(lessonKey);
  if (!lesson) return;
  lesson.uploadsPending += 1;
  videoUploading.value = true;
  videoProgress.value = 0;
  try {
    const cred = await getCourseUploadCredential({ ext });
    if (cred.mode === 'direct' && cred.host && cred.key) {
      const form = new FormData();
      form.append('key', cred.key);
      form.append('policy', cred.policy!);
      form.append('OSSAccessKeyId', cred.access_key_id!);
      form.append('Signature', cred.signature!);
      form.append('success_action_status', '200');
      form.append('file', file);
      await axios.post(cred.host, form, {
        onUploadProgress: (e) => {
          if (e.total)
            videoProgress.value = Math.round((e.loaded * 100) / e.total);
        },
      });
      const target = findLesson(lessonKey);
      if (target) target.video = cred.key;
    } else {
      const form = new FormData();
      form.append('file', file);
      const res = await axios.post(
        import.meta.env.VITE_HOST + '/v1/admin/course/video',
        form,
        {
          headers: {
            Authorization: 'Bearer ' + localStorage.getItem(TOKEN_KEY),
          },
          onUploadProgress: (e) => {
            if (e.total)
              videoProgress.value = Math.round((e.loaded * 100) / e.total);
          },
        },
      );
      if (res.data?.code !== 0) {
        throw new Error(res.data?.msg || t('course.upload.failed'));
      }
      const target = findLesson(lessonKey);
      if (target) target.video = res.data.data.video_url;
    }
    window.$message.success(t('course.list.videoUploadDone'));
  } catch (err: any) {
    const target = findLesson(lessonKey);
    if (target) target.videoName = '';
    window.$message.error(err?.message || t('course.list.videoUploadFailed'));
  } finally {
    videoUploading.value = false;
    const target = findLesson(lessonKey);
    if (target) target.uploadsPending = Math.max(0, target.uploadsPending - 1);
  }
};

const beforeAttachmentPick = async (data: any, lessonKey: string) => {
  const file: File | undefined = data.file?.file;
  if (!file) return false;
  const allowed = ['pdf', 'doc', 'docx', 'ppt', 'pptx', 'zip'];
  if (
    !allowed.includes((file.name.split('.').pop() || '').toLowerCase()) ||
    file.size > 100 * 1024 * 1024
  ) {
    window.$message.warning(t('course.list.attachmentInvalid'));
    return false;
  }
  const lesson = findLesson(lessonKey);
  if (!lesson) return false;
  attachmentUploading.value = true;
  lesson.uploadsPending += 1;
  try {
    const form = new FormData();
    form.append('type', 'attachment');
    form.append('file', file);
    const res = await axios.post(
      import.meta.env.VITE_HOST + '/v1/attachment',
      form,
      {
        headers: { Authorization: 'Bearer ' + localStorage.getItem(TOKEN_KEY) },
      },
    );
    if (res.data?.code !== 0)
      throw new Error(res.data?.msg || t('course.upload.failed'));
    const target = findLesson(lessonKey);
    if (target)
      target.attachments.push({
        name: file.name,
        url: res.data.data.content,
        sort: target.attachments.length,
      });
  } catch (err: any) {
    window.$message.error(err?.message || t('course.upload.failed'));
  } finally {
    attachmentUploading.value = false;
    const target = findLesson(lessonKey);
    if (target) target.uploadsPending = Math.max(0, target.uploadsPending - 1);
  }
  return false;
};

const openCourseModal = async (course?: CourseItem) => {
  let editableCourse = course;
  if (course?.id) {
    try {
      editableCourse = (await getCourse({ id: course.id })).course;
    } catch (_err) {
      window.$message.error(t('course.list.loadDetailFailed'));
      return;
    }
  }
  courseForm.id = editableCourse?.id || 0;
  courseForm.group_id = editableCourse?.group_id ?? null;
  courseForm.teacher_id = editableCourse?.teacher_id ?? null;
  courseForm.title = editableCourse?.title || '';
  courseForm.intro = editableCourse?.intro || '';
  lessons.value = editableCourse?.lessons?.length
    ? editableCourse.lessons.map((lesson) => ({
        ...lesson,
        video: '',
        attachments: [...(lesson.attachments || [])],
        clientKey: nextLessonKey(),
        uploadsPending: 0,
      }))
    : [newLesson(editableCourse?.title || '')];
  videoProgress.value = 0;
  coverBlob.value = null;
  coverUrl.value = '';
  coverPreview.value = editableCourse?.cover || '';
  if (editableCourse) {
    teacherOptions.value = [
      {
        label: `${editableCourse.teacher?.nickname} (@${editableCourse.teacher?.username})`,
        value: editableCourse.teacher_id,
      },
    ];
  } else {
    teacherOptions.value = [];
  }
  courseModalShow.value = true;
};

const saveCourse = async () => {
  if (!courseForm.group_id) {
    window.$message.warning(t('course.list.selectGroupRequired'));
    return;
  }
  if (!courseForm.title.trim()) {
    window.$message.warning(t('course.list.inputCourseTitle'));
    return;
  }
  if (!courseForm.teacher_id) {
    window.$message.warning(t('course.list.selectTeacherRequired'));
    return;
  }
  if (
    !lessons.value.length ||
    lessons.value.some((lesson) => !lesson.title.trim())
  ) {
    window.$message.warning(t('course.list.lessonTitleRequired'));
    return;
  }
  courseSaving.value = true;
  try {
    if (courseForm.id > 0) {
      await updateCourse({
        id: courseForm.id,
        group_id: courseForm.group_id,
        teacher_id: courseForm.teacher_id,
        title: courseForm.title.trim(),
        intro: courseForm.intro,
        video: undefined,
        cover: coverUrl.value || undefined,
        lessons: lessons.value.map((lesson, index) => ({
          ...lesson,
          sort: index,
        })),
      });
    } else {
      await createCourse({
        group_id: courseForm.group_id,
        teacher_id: courseForm.teacher_id,
        title: courseForm.title.trim(),
        intro: courseForm.intro,
        video: undefined,
        cover: coverUrl.value,
        lessons: lessons.value.map((lesson, index) => ({
          ...lesson,
          sort: index,
        })),
      });
    }
    window.$message.success(t('course.list.saveSuccess'));
    courseModalShow.value = false;
    loadGroups();
    if (searching.value) doSearch();
  } catch (_err) {
    // 错误提示已由拦截器弹出
  } finally {
    courseSaving.value = false;
  }
};

const execDeleteCourse = async (course: CourseItem) => {
  try {
    await deleteCourse({ id: course.id });
    window.$message.success(t('course.deleteSuccess'));
    loadGroups();
    if (searching.value) doSearch();
  } catch (_err) {
    // do nothing
  }
};

onMounted(() => {
  loadGroups();
});
</script>

<style lang="less" scoped>
.courses-wrap {
    padding: 0 0 20px;
}

.toolbar {
    display: flex;
    gap: 8px;
    padding: 12px 16px;
    align-items: center;

    .search-input {
        max-width: 320px;
    }
}

.section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 16px 8px;

    .section-title {
        font-size: 16px;
        font-weight: bold;

        .section-count {
            font-size: 12px;
            font-weight: normal;
            opacity: 0.6;
            margin-left: 8px;
        }
    }

    .section-ops {
        display: flex;
        gap: 12px;
        align-items: center;
    }
}

.course-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px;
    padding: 0 16px 8px;

    .group-empty {
        grid-column: 1 / -1;
        opacity: 0.5;
        padding: 16px 0;
        text-align: center;
    }
}

.drawer-list {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
}

.video-upload-wrap {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;

    .video-ready {
        color: #18a058;
        font-size: 12px;
    }
}

.lesson-editor { width: 100%; display: grid; gap: 12px; }
.lesson-card { display: grid; gap: 8px; padding: 12px; border: 1px solid var(--n-border-color); border-radius: 8px; }
.lesson-head, .lesson-attachment { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.lesson-attachment { padding: 4px 8px; background: rgba(127, 127, 127, .08); border-radius: 6px; }

.cover-wrap {
    display: flex;
    flex-direction: column;
    gap: 6px;

    .cover-preview {
        width: 240px;
        aspect-ratio: 16 / 9;
        object-fit: cover;
        border-radius: 6px;
        background: #000;
    }

    .cover-hint {
        font-size: 12px;
        opacity: 0.6;
    }
}

.empty-wrap {
    padding: 40px 0;
}

.load-end {
    display: block;
    text-align: center;
    color: #999;
    font-size: 13px;
    padding: 16px 0;
}
</style>
