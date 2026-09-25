<template>
    <div>
        <main-nav :title="course?.title || '课程详情'" :back="true" />

        <div v-if="loading" class="detail-loading">
            <n-spin size="large" />
        </div>

        <template v-else-if="course">
            <!-- 播放器(video.js, 签名地址异步到达后初始化) -->
            <div class="player-wrap">
                <video-player
                    v-if="videoUrl"
                    :src="videoUrl"
                    :poster="course.cover"
                    @play="onFirstPlay"
                />
                <div v-else class="player-loading">
                    <n-spin size="large" />
                </div>
            </div>

            <!-- 标题与数据 -->
            <div class="course-head">
                <div class="course-title">{{ course.title }}</div>
                <div class="course-meta">
                    <span>{{ course.play_count }} 次播放</span>
                    <span>{{ course.comment_count }} 条评论</span>
                    <span>{{ formatPrettyTime(course.created_on) }}</span>
                    <n-tag v-if="course.group_name" size="small" round>{{ course.group_name }}</n-tag>
                </div>
            </div>

            <!-- 老师与简介 -->
            <div class="teacher-wrap" v-if="course.teacher">
                <router-link
                    class="teacher-link"
                    :to="{ name: 'user', query: { s: course.teacher.username } }"
                >
                    <n-avatar round :size="44" :src="course.teacher.avatar" />
                    <div class="teacher-info">
                        <div class="teacher-name">{{ course.teacher.nickname }}</div>
                        <div class="teacher-username">@{{ course.teacher.username }}</div>
                    </div>
                </router-link>
                <n-tag size="small" type="warning" round>讲师</n-tag>
            </div>
            <div class="course-intro" v-if="course.intro">{{ course.intro }}</div>

            <!-- 评论区 -->
            <div class="comment-section">
                <div class="comment-title">{{ course.comment_count }} 条评论</div>
                <n-list bordered>
                    <n-list-item>
                        <course-compose-comment
                            :course-id="course.id"
                            @post-success="reloadComments"
                        />
                    </n-list-item>
                    <n-list-item v-for="comment in comments" :key="comment.id">
                        <course-comment-item
                            :comment="comment"
                            @reload="reloadComments"
                        />
                    </n-list-item>
                </n-list>
                <n-empty v-if="comments.length === 0" description="暂无评论" />
                <InfiniteLoading :key="commentsLoadKey" @infinite="onCommentsInfinite">
                    <template #complete><span class="load-end">没有更多评论了</span></template>
                </InfiniteLoading>
            </div>
        </template>

        <div v-else class="detail-loading">
            <n-empty size="large" description="课程不存在或已删除" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import InfiniteLoading from 'v3-infinite-loading';
import VideoPlayer from '@/components/video-player.vue';
import { formatPrettyTime } from '@/utils/formatTime';
import {
  getCourse,
  getCourseVideo,
  getCourseComments,
  playCourse,
  type CourseItem,
  type CourseComment,
} from '@/api/course';

const route = useRoute();
const courseId = Number(route.query.id || 0);

const loading = ref(true);
const course = ref<CourseItem | null>(null);
const videoUrl = ref('');
const comments = ref<CourseComment[]>([]);
const commentPage = ref(1);
const commentTotal = ref(0);
const commentLoading = ref(false);
// 自增key强制InfiniteLoading重挂载(重置内部状态), 评论加载全部经由@infinite触发
const commentsLoadKey = ref(0);
const pageSize = 20;
// 每次进入详情页只计一次播放
const playCounted = ref(false);

const loadCourse = async () => {
  loading.value = true;
  try {
    const res = await getCourse({ id: courseId });
    course.value = res.course;
  } catch (_err) {
    course.value = null;
  } finally {
    loading.value = false;
  }
};

const loadVideoUrl = async () => {
  try {
    const res = await getCourseVideo({ id: courseId });
    videoUrl.value = res.signed_url;
  } catch (_err) {
    window.$message.error('获取播放地址失败');
  }
};

const onFirstPlay = () => {
  if (playCounted.value || !course.value) return;
  playCounted.value = true;
  playCourse({ id: courseId })
    .then((res) => {
      if (course.value) course.value.play_count = res.play_count;
    })
    .catch(() => {});
};

// 返回是否还有更多(供InfiniteLoading决定loaded/complete)
const loadCommentsData = async (): Promise<boolean> => {
  if (commentLoading.value) return false;
  if (commentTotal.value > 0 && comments.value.length >= commentTotal.value) return false;
  commentLoading.value = true;
  try {
    const res = await getCourseComments({ id: courseId, page: commentPage.value, page_size: pageSize });
    comments.value = comments.value.concat(res.list || []);
    commentTotal.value = res.pager?.total_rows || 0;
    commentPage.value++;
    return comments.value.length < commentTotal.value;
  } catch (_err) {
    throw new Error('load failed');
  } finally {
    commentLoading.value = false;
  }
};
// InfiniteLoading 收尾: 必须调用 $state.loaded()/complete() 否则spinner不消失
const onCommentsInfinite = async ($state: any) => {
  try {
    const hasMore = await loadCommentsData();
    if (hasMore) {
      $state.loaded();
    } else {
      $state.complete();
    }
  } catch (_err) {
    $state.error();
  }
};

const reloadComments = () => {
  comments.value = [];
  commentPage.value = 1;
  commentTotal.value = 0;
  commentsLoadKey.value++;
  // 评论数可能因审核延迟变化 重新拉取详情
  loadCourse();
};

onMounted(() => {
  if (courseId > 0) {
    loadCourse();
    loadVideoUrl();
    // 评论由InfiniteLoading进入视口时自动加载
  } else {
    loading.value = false;
  }
});
</script>

<style lang="less" scoped>
.detail-loading {
    display: flex;
    justify-content: center;
    padding: 60px 0;
}

.player-wrap {
    width: 100%;
    border-radius: 8px;
    overflow: hidden;

    .player-loading {
        display: flex;
        align-items: center;
        justify-content: center;
        aspect-ratio: 16 / 9;
        background: #000;
    }
}

.course-head {
    padding: 14px 4px 0;

    .course-title {
        font-size: 18px;
        font-weight: bold;
        line-height: 1.5;
    }

    .course-meta {
        display: flex;
        align-items: center;
        gap: 14px;
        margin-top: 8px;
        font-size: 13px;
        opacity: 0.7;
    }
}

.teacher-wrap {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 14px 4px;

    .teacher-link {
        display: flex;
        align-items: center;
        gap: 10px;
        color: inherit;
        text-decoration: none;
    }

    .teacher-info {
        .teacher-name {
            font-size: 15px;
            font-weight: bold;
        }

        .teacher-username {
            font-size: 12px;
            opacity: 0.65;
        }
    }
}

.course-intro {
    padding: 0 4px 14px;
    font-size: 14px;
    line-height: 1.8;
    white-space: pre-wrap;
    word-break: break-all;
    opacity: 0.9;
}

.comment-section {
    padding-bottom: 20px;

    .comment-title {
        font-size: 15px;
        font-weight: bold;
        padding: 10px 4px;
    }

    .load-end {
        display: block;
        text-align: center;
        opacity: 0.5;
        font-size: 12px;
        padding: 12px 0;
    }
}
</style>
