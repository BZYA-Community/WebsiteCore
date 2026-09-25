<template>
    <div ref="wrapEl" class="video-player-wrap">
        <div ref="playerEl" class="video-player-box" :style="boxStyle"></div>
    </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import Artplayer from 'artplayer';

const props = withDefaults(
  defineProps<{
    src: string;
    poster?: string;
  }>(),
  {
    poster: '',
  },
);

const emit = defineEmits<{
  (e: 'play'): void;
}>();

const wrapEl = ref<HTMLDivElement>();
const playerEl = ref<HTMLDivElement>();
// 默认按16:9占位, 视频元数据加载后按实际比例重算(竖屏视频高度封顶居中)
const boxStyle = ref<{ width: string; height: string }>({ width: '100%', height: 'auto' });
let art: Artplayer | null = null;

// 播放器最大高度: 视口72%且不超过800px, 避免9:16竖屏视频把播放器撑出屏幕
const fitSize = () => {
  const wrap = wrapEl.value;
  if (!wrap) return;
  const containerW = wrap.clientWidth;
  const maxH = Math.min(window.innerHeight * 0.72, 800);
  const video = art?.video;
  const vw = video?.videoWidth || 0;
  const vh = video?.videoHeight || 0;
  let w = containerW;
  let h = (containerW * 9) / 16;
  if (vw > 0 && vh > 0) {
    h = (containerW * vh) / vw;
    if (h > maxH) {
      h = maxH;
      w = (h * vw) / vh;
    }
  } else if (h > maxH) {
    h = maxH;
  }
  boxStyle.value = { width: `${Math.round(w)}px`, height: `${Math.round(h)}px` };
};

const initPlayer = () => {
  if (!playerEl.value || !props.src || art) return;
  fitSize();
  // 注意: artplayer会校验option.poster必须为非空string, 为空时不能传该键
  const options: Parameters<typeof Artplayer>[0] = {
    container: playerEl.value,
    url: props.src,
    playbackRate: true,
    setting: true,
    fullscreen: true,
    fullscreenWeb: true,
    miniProgressBar: true,
    playsInline: true,
  };
  if (props.poster) {
    options.poster = props.poster;
  }
  art = new Artplayer(options);
  art.on('play', () => emit('play'));
  art.on('video:loadedmetadata', fitSize);
};

watch(
  () => props.src,
  (src) => {
    if (!src) return;
    if (!art) {
      initPlayer();
    } else {
      art.url = src;
    }
  },
);
watch(
  () => props.poster,
  (poster) => {
    if (art && poster) art.poster = poster;
  },
);

const onResize = () => fitSize();

onMounted(() => {
  initPlayer();
  window.addEventListener('resize', onResize);
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize);
  art?.destroy();
  art = null;
});
</script>

<style lang="less" scoped>
.video-player-wrap {
    width: 100%;
    display: flex;
    justify-content: center;
    background: #000;
    border-radius: 8px;
    overflow: hidden;

    .video-player-box {
        width: 100%;
    }
}
</style>
