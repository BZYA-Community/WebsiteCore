<template>
    <div class="course-card" @click="goDetail">
        <div class="cover-box">
            <img v-if="course.cover" :src="course.cover" class="cover" loading="lazy" />
            <div v-else class="cover cover-placeholder">
                <n-icon size="32" :depth="3"><play-circle-outline /></n-icon>
            </div>
            <span class="play-count">
                <n-icon size="12"><play-outline /></n-icon>
                {{ course.play_count }}
            </span>
        </div>
        <div class="info">
            <div class="title" :title="course.title">{{ course.title }}</div>
            <div class="meta">
                <span class="teacher">{{ course.teacher?.nickname || '未知老师' }}</span>
                <span class="comments">{{ course.comment_count }} 评论</span>
            </div>
        </div>
        <div v-if="isAdmin" class="admin-ops" @click.stop>
            <n-button text size="tiny" @click="emit('edit')">编辑</n-button>
            <n-popconfirm
                negative-text="取消"
                positive-text="删除"
                @positive-click="emit('delete')"
            >
                <template #trigger>
                    <n-button text size="tiny" type="error">删除</n-button>
                </template>
                确定删除课程「{{ course.title }}」？将同时删除其全部评论，不可恢复。
            </n-popconfirm>
        </div>
    </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import { PlayCircleOutline, PlayOutline } from '@vicons/ionicons5';
import type { CourseItem } from '@/api/course';

const props = withDefaults(
  defineProps<{
    course: CourseItem;
    isAdmin?: boolean;
  }>(),
  {
    isAdmin: false,
  },
);

const emit = defineEmits<{
  (e: 'edit'): void;
  (e: 'delete'): void;
}>();

const router = useRouter();
const goDetail = () => {
  router.push({
    name: 'course',
    query: { id: props.course.id },
  });
};
</script>

<style lang="less" scoped>
.course-card {
    position: relative;
    cursor: pointer;
    border-radius: 8px;
    overflow: hidden;
    transition: transform 0.15s ease;

    &:hover {
        transform: translateY(-2px);
    }

    .cover-box {
        position: relative;
        aspect-ratio: 16 / 9;
        background: #000;
        border-radius: 8px;
        overflow: hidden;

        .cover {
            width: 100%;
            height: 100%;
            object-fit: cover;
            display: block;
        }

        .cover-placeholder {
            display: flex;
            align-items: center;
            justify-content: center;
            color: #fff;
        }

        .play-count {
            position: absolute;
            left: 8px;
            bottom: 6px;
            color: #fff;
            font-size: 12px;
            display: flex;
            align-items: center;
            gap: 2px;
            text-shadow: 0 1px 2px rgba(0, 0, 0, 0.8);
        }
    }

    .info {
        padding: 6px 2px 0;

        .title {
            font-size: 14px;
            line-height: 1.4;
            display: -webkit-box;
            -webkit-line-clamp: 2;
            -webkit-box-orient: vertical;
            overflow: hidden;
        }

        .meta {
            display: flex;
            justify-content: space-between;
            font-size: 12px;
            opacity: 0.65;
            margin-top: 4px;
        }
    }

    .admin-ops {
        position: absolute;
        top: 6px;
        right: 6px;
        display: flex;
        gap: 6px;
        padding: 2px 6px;
        border-radius: 4px;
        background: rgba(255, 255, 255, 0.9);
    }
}

.dark {
    .course-card {
        .admin-ops {
            background: rgba(24, 24, 28, 0.9);
        }
    }
}
</style>
