<template>
    <div>
        <main-nav :title="title" />

        <div class="courses-wrap">
            <!-- 搜索与管理工具栏 -->
            <div class="toolbar">
                <n-input
                    v-model:value="keyword"
                    class="search-input"
                    placeholder="搜索课程标题 / 简介"
                    clearable
                    @keyup.enter="doSearch"
                    @clear="clearSearch"
                >
                    <template #prefix>
                        <n-icon><search-outline /></n-icon>
                    </template>
                </n-input>
                <n-button type="primary" secondary round @click="doSearch">搜索</n-button>
                <template v-if="userInfo.is_admin">
                    <n-button secondary round @click="openGroupModal()">新建分组</n-button>
                    <n-button secondary round type="info" @click="openCourseModal()">新建课程</n-button>
                </template>
            </div>

            <!-- 搜索结果(扁平分页) -->
            <template v-if="searching">
                <div class="section-header">
                    <span class="section-title">搜索「{{ searchKeyword }}」的结果</span>
                    <n-button text size="small" @click="clearSearch">返回分组</n-button>
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
                <n-empty v-if="!searchLoading && searchList.length === 0" description="没有找到相关课程" />
                <InfiniteLoading @infinite="loadSearchNext">
                    <template #complete><span></span></template>
                </InfiniteLoading>
            </template>

            <!-- 分组浏览 -->
            <template v-else>
                <div v-if="groups.length === 0 && !loadingGroups" class="empty-wrap">
                    <n-empty size="large" description="暂无课程分组" />
                </div>
                <div v-for="group in groups" :key="group.id" class="group-section">
                    <div class="section-header">
                        <span class="section-title">
                            {{ group.name }}
                            <span class="section-count">{{ group.course_count }} 门课程</span>
                        </span>
                        <div class="section-ops">
                            <n-button
                                v-if="group.course_count > groupPreviewSize"
                                text
                                size="small"
                                type="primary"
                                @click="enterGroup(group.id)"
                            >
                                查看全部
                            </n-button>
                            <template v-if="userInfo.is_admin">
                                <n-button text size="small" @click="openGroupModal(group)">编辑</n-button>
                                <n-popconfirm
                                    negative-text="取消"
                                    positive-text="删除"
                                    @positive-click="execDeleteGroup(group)"
                                >
                                    <template #trigger>
                                        <n-button text size="small" type="error">删除</n-button>
                                    </template>
                                    删除分组「{{ group.name }}」？(分组下有课程时不可删除)
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
                            该分组暂无课程
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
                <n-empty v-if="!drawerLoading && drawerList.length === 0" description="该分组暂无课程" />
                <InfiniteLoading @infinite="loadDrawerNext">
                    <template #complete><span></span></template>
                </InfiniteLoading>
            </n-drawer-content>
        </n-drawer>

        <!-- 分组编辑弹窗 -->
        <n-modal v-model:show="groupModalShow" preset="card" :title="groupForm.id > 0 ? '编辑分组' : '新建分组'" style="width: 420px">
            <n-form label-placement="left" label-width="80">
                <n-form-item label="分组名称" required>
                    <n-input v-model:value="groupForm.name" maxlength="64" show-count placeholder="分组名称" />
                </n-form-item>
                <n-form-item label="排序">
                    <n-input-number v-model:value="groupForm.sort" :min="0" placeholder="越小越靠前" />
                </n-form-item>
            </n-form>
            <template #footer>
                <n-button type="primary" :loading="groupSaving" @click="saveGroup">保存</n-button>
            </template>
        </n-modal>

        <!-- 课程编辑弹窗 -->
        <n-modal v-model:show="courseModalShow" preset="card" :title="courseForm.id > 0 ? '编辑课程' : '新建课程'" style="width: 640px">
            <n-form label-placement="left" label-width="80">
                <n-form-item label="所属分组" required>
                    <n-select
                        v-model:value="courseForm.group_id"
                        :options="groupOptions"
                        placeholder="选择分组"
                    />
                </n-form-item>
                <n-form-item label="课程标题" required>
                    <n-input v-model:value="courseForm.title" maxlength="128" show-count placeholder="课程标题" />
                </n-form-item>
                <n-form-item label="课程简介">
                    <n-input
                        v-model:value="courseForm.intro"
                        type="textarea"
                        maxlength="2000"
                        show-count
                        :autosize="{ minRows: 3, maxRows: 8 }"
                        placeholder="课程简介(搜索时可被检索)"
                    />
                </n-form-item>
                <n-form-item label="课程老师" required>
                    <n-select
                        v-model:value="courseForm.teacher_id"
                        filterable
                        remote
                        :options="teacherOptions"
                        :loading="teacherLoading"
                        placeholder="输入用户名/昵称搜索"
                        @search="searchTeachers"
                        @focus="searchTeachers('')"
                    />
                </n-form-item>
                <n-form-item :label="courseForm.id > 0 ? '更换视频' : '课程视频'" :required="courseForm.id === 0">
                    <div class="video-upload-wrap">
                        <n-upload
                            :show-file-list="false"
                            :custom-request="noopUpload"
                            @before-upload="beforeVideoPick"
                        >
                            <n-button secondary>
                                {{ videoName || '选择视频文件(mp4/mov, ≤500MB)' }}
                            </n-button>
                        </n-upload>
                        <n-progress
                            v-if="videoUploading"
                            type="line"
                            :percentage="videoProgress"
                            :show-indicator="true"
                        />
                        <span v-if="videoReady" class="video-ready">✓ 视频已{{ courseForm.id > 0 ? '更换' : '上传' }}</span>
                    </div>
                </n-form-item>
                <n-form-item label="课程封面">
                    <div class="cover-wrap">
                        <img v-if="coverPreview" :src="coverPreview" class="cover-preview" alt="封面预览" />
                        <span class="cover-hint">封面自动截取视频画面生成，重新选择视频会重新截取</span>
                    </div>
                </n-form-item>
            </n-form>
            <template #footer>
                <n-button type="primary" :loading="courseSaving" :disabled="videoUploading" @click="saveCourse">保存</n-button>
            </template>
        </n-modal>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import { useStoreUser } from '@/store/user';
import { storeToRefs } from 'pinia';
import axios from 'axios';
import { SearchOutline } from '@vicons/ionicons5';
import CourseCard from '@/components/course/course-card.vue';
import {
  getCourseGroups,
  getCourseList,
  createCourseGroup,
  updateCourseGroup,
  deleteCourseGroup,
  createCourse,
  updateCourse,
  deleteCourse,
  getCourseUploadCredential,
  type CourseGroup,
  type CourseItem,
} from '@/api/course';
import { Api } from '@/utils/request';
import { TOKEN_KEY } from '@/store/user';
import InfiniteLoading from 'v3-infinite-loading';
import type { UploadCustomRequestOptions } from 'naive-ui';

const title = '课程';
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
    const res = await getCourseList({ group_id: groupId, page: 1, page_size: groupPreviewSize });
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
  loadSearchNext();
};
const clearSearch = () => {
  searching.value = false;
  keyword.value = '';
  searchList.value = [];
};
const loadSearchNext = async () => {
  if (searchLoading.value) return;
  if (searchTotal.value > 0 && searchList.value.length >= searchTotal.value) return;
  searchLoading.value = true;
  try {
    const res = await getCourseList({ keyword: searchKeyword.value, page: searchPage.value, page_size: pageSize });
    searchList.value = searchList.value.concat(res.list || []);
    searchTotal.value = res.pager?.total_rows || 0;
    searchPage.value++;
  } catch (_err) {
    // do nothing
  } finally {
    searchLoading.value = false;
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

const enterGroup = (groupId: number) => {
  const g = groups.value.find((i) => i.id === groupId);
  drawerGroupId.value = groupId;
  drawerGroupName.value = g?.name || '';
  drawerList.value = [];
  drawerPage.value = 1;
  drawerTotal.value = 0;
  groupDrawerShow.value = true;
  loadDrawerNext();
};
const loadDrawerNext = async () => {
  if (drawerLoading.value) return;
  if (drawerTotal.value > 0 && drawerList.value.length >= drawerTotal.value) return;
  drawerLoading.value = true;
  try {
    const res = await getCourseList({ group_id: drawerGroupId.value, page: drawerPage.value, page_size: pageSize });
    drawerList.value = drawerList.value.concat(res.list || []);
    drawerTotal.value = res.pager?.total_rows || 0;
    drawerPage.value++;
  } catch (_err) {
    // do nothing
  } finally {
    drawerLoading.value = false;
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
    window.$message.warning('请输入分组名称');
    return;
  }
  groupSaving.value = true;
  try {
    if (groupForm.id > 0) {
      await updateCourseGroup({ id: groupForm.id, name: groupForm.name.trim(), sort: groupForm.sort });
    } else {
      await createCourseGroup({ name: groupForm.name.trim(), sort: groupForm.sort });
    }
    window.$message.success('保存成功');
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
    window.$message.success('删除成功');
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

const groupOptions = computed(() =>
  groups.value.map((g) => ({ label: g.name, value: g.id })),
);

// 老师远程搜索(复用管理员用户搜索)
const teacherOptions = ref<{ label: string; value: number }[]>([]);
const teacherLoading = ref(false);
const searchTeachers = async (k: string) => {
  teacherLoading.value = true;
  try {
    const res = await Api.v1.admin.get.user.list({ keyword: k, page: 1, page_size: 20 });
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
const videoName = ref('');
const videoUploading = ref(false);
const videoProgress = ref(0);
const videoReady = ref(false);
const videoKeyOrUrl = ref('');
// 封面(canvas截帧)
const coverBlob = ref<Blob | null>(null);
const coverPreview = ref('');
const coverUrl = ref('');

const noopUpload = (_options: UploadCustomRequestOptions) => {
  // 仅用于选择文件, 实际上传走 beforeVideoPick 自定义流程
};

const beforeVideoPick = async (data: any) => {
  const file: File | undefined = data.file?.file;
  if (!file) return false;
  const ext = '.' + (file.name.split('.').pop() || '').toLowerCase();
  if (!['.mp4', '.mov'].includes(ext)) {
    window.$message.warning('课程视频仅允许 mp4/mov 格式');
    return false;
  }
  if (file.size > 1024 * 1024 * 500) {
    window.$message.warning('课程视频最大允许500MB');
    return false;
  }
  videoName.value = file.name;
  videoReady.value = false;
  // 用本地文件截帧生成封面(避免OSS跨域污染canvas)
  captureCover(file);
  await uploadVideo(file, ext);
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
      canvas.getContext('2d')?.drawImage(video, 0, 0, canvas.width, canvas.height);
      canvas.toBlob((b) => {
        cleanup();
        if (!b) return;
        coverBlob.value = b;
        if (coverPreview.value) URL.revokeObjectURL(coverPreview.value);
        coverPreview.value = URL.createObjectURL(b);
        // 封面即时上传到图床(复用公开图片通道)
        uploadCover(b);
      }, 'image/jpeg', 0.85);
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
      { headers: { Authorization: 'Bearer ' + localStorage.getItem(TOKEN_KEY) } },
    );
    if (res.data?.code === 0) {
      coverUrl.value = res.data.data.content;
    }
  } catch (_err) {
    window.$message.warning('封面生成失败, 可稍后重试或编辑课程更换');
  }
};

// 视频上传: 先取凭证, direct=浏览器直传AliOSS / proxy=后端中转
const uploadVideo = async (file: File, ext: string) => {
  videoUploading.value = true;
  videoProgress.value = 0;
  try {
    const cred = await getCourseUploadCredential({ ext });
    if (cred.mode === 'direct' && cred.host && cred.key) {
      const form = new FormData();
      form.append('key', cred.key);
      form.append('policy', cred.policy!);
      form.append('OSSAccessKeyId', cred.access_key_id!);
      form.append('success_action_status', '200');
      form.append('file', file);
      await axios.post(cred.host, form, {
        onUploadProgress: (e) => {
          if (e.total) videoProgress.value = Math.round((e.loaded * 100) / e.total);
        },
      });
      videoKeyOrUrl.value = cred.key;
    } else {
      const form = new FormData();
      form.append('file', file);
      const res = await axios.post(
        import.meta.env.VITE_HOST + '/v1/admin/course/video',
        form,
        {
          headers: { Authorization: 'Bearer ' + localStorage.getItem(TOKEN_KEY) },
          onUploadProgress: (e) => {
            if (e.total) videoProgress.value = Math.round((e.loaded * 100) / e.total);
          },
        },
      );
      if (res.data?.code !== 0) {
        throw new Error(res.data?.msg || '上传失败');
      }
      videoKeyOrUrl.value = res.data.data.video_url;
    }
    videoReady.value = true;
    window.$message.success('视频上传完成');
  } catch (err: any) {
    videoName.value = '';
    window.$message.error(err?.message || '视频上传失败');
  } finally {
    videoUploading.value = false;
  }
};

const openCourseModal = (course?: CourseItem) => {
  courseForm.id = course?.id || 0;
  courseForm.group_id = course?.group_id ?? null;
  courseForm.teacher_id = course?.teacher_id ?? null;
  courseForm.title = course?.title || '';
  courseForm.intro = course?.intro || '';
  videoName.value = '';
  videoReady.value = false;
  videoProgress.value = 0;
  videoKeyOrUrl.value = '';
  coverBlob.value = null;
  coverUrl.value = '';
  coverPreview.value = course?.cover || '';
  if (course) {
    teacherOptions.value = [
      {
        label: `${course.teacher?.nickname} (@${course.teacher?.username})`,
        value: course.teacher_id,
      },
    ];
  } else {
    teacherOptions.value = [];
  }
  courseModalShow.value = true;
};

const saveCourse = async () => {
  if (!courseForm.group_id) {
    window.$message.warning('请选择分组');
    return;
  }
  if (!courseForm.title.trim()) {
    window.$message.warning('请输入课程标题');
    return;
  }
  if (!courseForm.teacher_id) {
    window.$message.warning('请选择课程老师');
    return;
  }
  if (courseForm.id === 0 && !videoKeyOrUrl.value) {
    window.$message.warning('请上传课程视频');
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
        video: videoKeyOrUrl.value || undefined,
        cover: coverUrl.value || undefined,
      });
    } else {
      await createCourse({
        group_id: courseForm.group_id,
        teacher_id: courseForm.teacher_id,
        title: courseForm.title.trim(),
        intro: courseForm.intro,
        video: videoKeyOrUrl.value,
        cover: coverUrl.value,
      });
    }
    window.$message.success('保存成功');
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
    window.$message.success('删除成功');
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
</style>
