<template>
    <div v-if="props.videos.length > 0">
        <n-grid :x-gap="4" :y-gap="4" :cols="full ? 1 : 5">
            <n-gi :span="full ? 1 : 3">
                <video-player @click.stop v-for="video in props.videos" :key="video.id" :src="video.content" />
            </n-gi>
        </n-grid>
    </div>
</template>

<script setup lang="ts">
import VideoPlayer from '@/components/video-player.vue';

const props = withDefaults(
  defineProps<{
    videos: Item.PostItemProps[];
    full?: boolean;
  }>(),
  {
    videos: () => [],
    full: false,
  },
);
</script>
