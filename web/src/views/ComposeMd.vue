<template>
    <div>
        <main-nav title="发布长文" :back="true" />

        <n-card size="small" class="main-content-wrap compose-md-card">
            <div class="compose-md-editor-wrap">
                <md-editor
                    class="compose-md-page-editor"
                    :model-value="content"
                    :theme="editorTheme"
                    language="zh-CN"
                    placeholder="支持 Markdown：井号后加空格是标题，#话题# 或 #话题 加空格是话题标签"
                    :toolbars-exclude="mdToolbarsExclude"
                    no-mermaid
                    no-katex
                    @update:model-value="changeContent"
                />
                <div class="draft-tip">草稿自动保存，仅保留最近一次未发布的内容</div>
            </div>

            <n-upload
                ref="uploadRef"
                abstract
                list-type="image"
                :multiple="true"
                :max="9"
                :action="uploadGateway"
                :headers="{
                    Authorization: uploadToken,
                }"
                :data="{
                    type: uploadType,
                }"
                :file-list="fileQueue"
                @before-upload="beforeUpload"
                @finish="finishUpload"
                @error="failUpload"
                @remove="removeUpload"
                @update:file-list="updateUpload"
            >
                <div class="compose-md-toolbar">
                    <div class="attachment">
                        <n-upload-trigger #="{ handleClick }" abstract>
                            <n-button
                                :disabled="
                                    (fileQueue.length > 0 &&
                                        uploadType === 'public/video') ||
                                    fileQueue.length === 9
                                "
                                @click="
                                    () => {
                                        setUploadType('public/image');
                                        handleClick();
                                    }
                                "
                                quaternary
                                circle
                                type="primary"
                            >
                                <template #icon>
                                    <n-icon
                                        size="20"
                                        color="var(--primary-color)"
                                    >
                                        <image-outline />
                                    </n-icon>
                                </template>
                            </n-button>
                        </n-upload-trigger>

                        <n-upload-trigger
                          v-if="profile.allowTweetVideo"
                          #="{ handleClick }" abstract>
                            <n-button
                                :disabled="
                                    (fileQueue.length > 0 &&
                                        uploadType !== 'public/video') ||
                                    fileQueue.length === 9
                                "
                                @click="
                                    () => {
                                        setUploadType('public/video');
                                        handleClick();
                                    }
                                "
                                quaternary
                                circle
                                type="primary"
                            >
                                <template #icon>
                                    <n-icon
                                        size="20"
                                        color="var(--primary-color)"
                                    >
                                        <videocam-outline />
                                    </n-icon>
                                </template>
                            </n-button>
                        </n-upload-trigger>

                        <n-upload-trigger
                          v-if="profile.allowTweetAttachment"
                          #="{ handleClick }" abstract>
                            <n-button
                                :disabled="
                                    (fileQueue.length > 0 &&
                                        uploadType === 'public/video') ||
                                    fileQueue.length === 9
                                "
                                @click="
                                    () => {
                                        setUploadType('attachment');
                                        handleClick();
                                    }
                                "
                                quaternary
                                circle
                                type="primary"
                            >
                                <template #icon>
                                    <n-icon
                                        size="20"
                                        color="var(--primary-color)"
                                    >
                                        <attach-outline />
                                    </n-icon>
                                </template>
                            </n-button>
                        </n-upload-trigger>

                        <n-button
                            quaternary
                            circle
                            type="primary"
                            @click.stop="switchLink"
                        >
                            <template #icon>
                                <n-icon size="20" color="var(--primary-color)">
                                    <compass-outline />
                                </n-icon>
                            </template>
                        </n-button>

                        <n-button
                            v-if="allowTweetVisibility"
                            quaternary
                            circle
                            type="primary"
                            @click.stop="switchEye"
                        >
                            <template #icon>
                                <n-icon size="20" color="var(--primary-color)">
                                    <eye-outline />
                                </n-icon>
                            </template>
                        </n-button>
                    </div>

                    <div class="submit-wrap">
                        <n-tooltip trigger="hover" placement="bottom">
                            <template #trigger>
                                <n-progress
                                    class="text-statistic"
                                    type="circle"
                                    :show-indicator="false"
                                    status="success"
                                    :stroke-width="10"
                                    :percentage="
                                        (content.length / MD_MAX_LENGTH) * 100
                                    "
                                />
                            </template>
                            已输入{{ content.length }}字
                        </n-tooltip>

                        <n-button
                            :loading="submitting"
                            @click="submitPost"
                            type="primary"
                            secondary
                            round
                        >
                            发布
                        </n-button>
                    </div>
                </div>

                <div class="attachment-list-wrap">
                    <n-upload-file-list />
                </div>
            </n-upload>

            <div class="eye-wrap" v-if="showEyeSet">
                <n-radio-group v-model:value="visitType" name="radiogroup">
                    <n-space>
                        <n-radio
                            v-for="visit in visibilities"
                            :key="visit.value"
                            :value="visit.value"
                            :label="visit.label"
                        />
                    </n-space>
                </n-radio-group>
            </div>

            <div class="link-wrap" v-if="showLinkSet">
                <n-dynamic-input
                    v-model:value="links"
                    placeholder="请输入以http(s)://开头的链接"
                    :min="0"
                    :max="3"
                >
                    <template #create-button-default> 创建链接 </template>
                </n-dynamic-input>
            </div>
        </n-card>
    </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { storeToRefs } from 'pinia';
import { debounce } from 'lodash';
import { useStoreMain } from '@/store/main';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { useStoreProfile } from '@/store/profile';

import {
  ImageOutline,
  VideocamOutline,
  AttachOutline,
  CompassOutline,
  EyeOutline,
} from '@vicons/ionicons5';
import { MdEditor } from 'md-editor-v3';
import type { ToolbarNames } from 'md-editor-v3';
import 'md-editor-v3/lib/style.css';
import { createPost } from '@/api/post';
import { parsePostTag } from '@/utils/content';
import { MD_MAX_LENGTH, mdTheme } from '@/utils/markdown';
import { userInfo as fetchUserInfo } from '@/api/auth';
import { isZipFile } from '@/utils/isZipFile';
import type { UploadFileInfo, UploadInst } from 'naive-ui';
import { VisibilityEnum, PostItemTypeEnum } from '@/utils/IEnum';

const MD_DRAFT_KEY = 'paopao-md-draft';

const router = useRouter();
const storeMain = useStoreMain();
const storeUser = useStoreUser();
const storeProfile = useStoreProfile();
const { theme } = storeToRefs(storeMain);
const { userInfo } = storeToRefs(storeUser);
const { profile } = storeToRefs(storeProfile);

const editorTheme = computed(() => mdTheme(theme.value));
// 精简工具栏: 去掉依赖外部CDN或与站点能力重复的项
const mdToolbarsExclude: ToolbarNames[] = [
  'mermaid',
  'katex',
  'image',
  'github',
  'htmlPreview',
  'fullscreen',
];

const submitting = ref(false);
const showLinkSet = ref(false);
const showEyeSet = ref(false);
const content = ref('');
const links = ref([]);

const uploadRef = ref<UploadInst>();
const uploadType = ref('public/image');
const fileQueue = ref<UploadFileInfo[]>([]);
const imageContents = ref<Item.CommentItemProps[]>([]);
const videoContents = ref<Item.CommentItemProps[]>([]);
const attachmentContents = ref<Item.AttachmentProps[]>([]);
const visitType = ref<VisibilityEnum>(VisibilityEnum.PUBLIC);
const defaultVisitType = ref<VisibilityEnum>(VisibilityEnum);

const allowTweetVisibility = ref(
  import.meta.env.VITE_ALLOW_TWEET_VISIBILITY.toLowerCase() === 'true',
);
const uploadGateway = import.meta.env.VITE_HOST + '/v1/attachment';

const uploadToken = computed(() => {
  return 'Bearer ' + localStorage.getItem(TOKEN_KEY);
});

const visibilities = computed(() => {
  let res = [
    { value: VisibilityEnum.PUBLIC, label: '公开' },
    { value: VisibilityEnum.PRIVATE, label: '私密' },
    { value: VisibilityEnum.Following, label: '关注可见' },
  ];
  if (profile.value.useFriendship) {
    res.push({ value: VisibilityEnum.FRIEND, label: '好友可见' });
  }
  return res;
});

const switchLink = () => {
  showLinkSet.value = !showLinkSet.value;
  if (showLinkSet.value && showEyeSet.value) {
    showEyeSet.value = false;
  }
};

const switchEye = () => {
  showEyeSet.value = !showEyeSet.value;
  if (showEyeSet.value && showLinkSet.value) {
    showLinkSet.value = false;
  }
};

// 草稿自动保存(防误退出丢失长文) 发布成功后清除
const saveDraft = debounce(() => {
  if (content.value.trim().length > 0) {
    localStorage.setItem(MD_DRAFT_KEY, content.value);
  } else {
    localStorage.removeItem(MD_DRAFT_KEY);
  }
}, 500);

const changeContent = (v: string) => {
  if (v.length > MD_MAX_LENGTH) {
    content.value = v.substring(0, MD_MAX_LENGTH);
  } else {
    content.value = v;
  }
  saveDraft();
};

const setUploadType = (type: string) => {
  uploadType.value = type;
};

const updateUpload = (list: UploadFileInfo[]) => {
  for (let i = 0; i < list.length; i++) {
    var name = list[i].name;
    var basename: string = name.split('.').slice(0, -1).join('.');
    var ext: string = name.split('.').pop()!;
    if (basename.length > 30) {
      list[i].name =
        basename.substring(0, 18) +
        '...' +
        basename.substring(basename.length - 9) +
        '.' +
        ext;
    }
  }
  fileQueue.value = list;
};
const beforeUpload = async (data: any) => {
  // 图片类型校验
  if (
    uploadType.value === 'public/image' &&
    ![
      'image/webp',
      'image/png',
      'image/jpg',
      'image/jpeg',
      'image/gif',
    ].includes(data.file.file?.type)
  ) {
    window.$message.warning('图片仅允许 webp/png/jpg/gif 格式');
    return false;
  }

  if (uploadType.value === 'image' && data.file.file?.size > 10485760) {
    window.$message.warning('图片大小不能超过10MB');
    return false;
  }

  // 视频类型校验
  if (
    uploadType.value === 'public/video' &&
    !['video/mp4', 'video/quicktime'].includes(data.file.file?.type)
  ) {
    window.$message.warning('视频仅允许 mp4/mov 格式');
    return false;
  }

  if (uploadType.value === 'public/video' && data.file.file?.size > 104857600) {
    window.$message.warning('视频大小不能超过100MB');
    return false;
  }
  // 附件类型校验
  if (uploadType.value === 'attachment' && !(await isZipFile(data.file.file))) {
    window.$message.warning('附件仅允许 zip 格式');
    return false;
  }

  if (uploadType.value === 'attachment' && data.file.file?.size > 104857600) {
    window.$message.warning('附件大小不能超过100MB');
    return false;
  }

  return true;
};
const finishUpload = ({ file, event }: any): any => {
  try {
    let data = JSON.parse(event.target?.response);

    if (data.code === 0) {
      if (uploadType.value === 'public/image') {
        imageContents.value.push({
          id: file.id,
          content: data.data.content,
        } as Item.CommentItemProps);
      }
      if (uploadType.value === 'public/video') {
        videoContents.value.push({
          id: file.id,
          content: data.data.content,
        } as Item.CommentItemProps);
      }
      if (uploadType.value === 'attachment') {
        attachmentContents.value.push({
          id: file.id,
          content: data.data.content,
        } as Item.AttachmentProps);
      }
    }
  } catch (error) {
    window.$message.error('上传失败');
  }
};
const failUpload = ({ file, event }: any): any => {
  try {
    let data = JSON.parse(event.target?.response);

    if (data.code !== 0) {
      let errMsg = data.msg || '上传失败';
      if (data.details && data.details.length > 0) {
        data.details.map((detail: string) => {
          errMsg += ':' + detail;
        });
      }
      window.$message.error(errMsg);
    }
  } catch (error) {
    window.$message.error('上传失败');
  }
};
const removeUpload = ({ file }: any) => {
  let idx = imageContents.value.findIndex((item) => item.id === file.id);
  if (idx > -1) {
    imageContents.value.splice(idx, 1);
  }
  idx = videoContents.value.findIndex((item) => item.id === file.id);
  if (idx > -1) {
    videoContents.value.splice(idx, 1);
  }
  idx = attachmentContents.value.findIndex((item) => item.id === file.id);
  if (idx > -1) {
    attachmentContents.value.splice(idx, 1);
  }
};

// 发布Markdown长文
const submitPost = () => {
  if (content.value.trim().length === 0) {
    window.$message.warning('请输入内容哦');
    return;
  }

  // 解析用户at及tag
  let { tags, users } = parsePostTag(content.value);

  const contents = [];
  let sort = 100;

  contents.push({
    content: content.value,
    type: PostItemTypeEnum.MARKDOWN, // Markdown长文
    sort,
  });

  imageContents.value.map((img) => {
    sort++;
    contents.push({
      content: img.content,
      type: PostItemTypeEnum.IMAGEURL, // 图片
      sort,
    });
  });
  videoContents.value.map((video) => {
    sort++;
    contents.push({
      content: video.content,
      type: PostItemTypeEnum.VIDEOURL, // 视频
      sort,
    });
  });
  attachmentContents.value.map((attachment) => {
    sort++;
    contents.push({
      content: attachment.content,
      type: PostItemTypeEnum.ATTACHMENT, // 附件
      sort,
    });
  });
  if (links.value.length > 0) {
    links.value.map((link) => {
      sort++;
      contents.push({
        content: link,
        type: PostItemTypeEnum.LINKURL, // 链接
        sort,
      });
    });
  }

  submitting.value = true;
  createPost({
    contents,
    tags: Array.from(new Set(tags)),
    users: Array.from(new Set(users)),
    visibility: visitType.value,
  })
    .then((res) => {
      if (res.audit_status === 0) {
        window.$message.success('发布成功，内容审核通过后对他人可见');
      } else {
        window.$message.success('发布成功');
      }
      submitting.value = false;

      // 置空并清除草稿
      localStorage.removeItem(MD_DRAFT_KEY);
      showLinkSet.value = false;
      showEyeSet.value = false;
      uploadRef.value?.clear();
      fileQueue.value = [];
      content.value = '';
      links.value = [];
      imageContents.value = [];
      videoContents.value = [];
      attachmentContents.value = [];
      visitType.value = defaultVisitType.value;

      // 回广场并刷新
      router.replace('/');
      setTimeout(() => {
        storeMain.doRefresh();
      }, 50);
    })
    .catch((err) => {
      submitting.value = false;
    });
};

const ensureLogin = async () => {
  if (!localStorage.getItem(TOKEN_KEY) && userInfo.value.id === 0) {
    storeMain.triggerAuth(true);
    storeMain.triggerAuthKey('signin');
    router.replace({
      name: 'home',
    });
    return false;
  }

  if (userInfo.value.id === 0) {
    try {
      const currentUser = await fetchUserInfo();
      storeUser.updateUserinfo(currentUser);
    } catch (_err) {
      storeUser.userLogout();
      router.replace({
        name: 'home',
      });
      return false;
    }
  }

  return true;
};

onMounted(async () => {
  const allowed = await ensureLogin();
  if (!allowed) {
    return;
  }

  const defaultVisibility = profile.value.defaultTweetVisibility;
  if (profile.value.useFriendship && defaultVisibility === 'friend') {
    defaultVisitType.value = VisibilityEnum.FRIEND;
  } else if (defaultVisibility === 'following') {
    defaultVisitType.value = VisibilityEnum.Following;
  } else if (defaultVisibility === 'public') {
    defaultVisitType.value = VisibilityEnum.PUBLIC;
  } else {
    defaultVisitType.value = VisibilityEnum.PRIVATE;
  }
  visitType.value = defaultVisitType.value;

  // 恢复未发布草稿
  const draft = localStorage.getItem(MD_DRAFT_KEY);
  if (draft && draft.trim().length > 0) {
    content.value = draft;
    window.$message.info('已恢复上次未发布的草稿');
  }
});
</script>

<style lang="less" scoped>
.compose-md-card {
    margin-top: -1px;
    border-radius: 0;

    .compose-md-editor-wrap {
        .compose-md-page-editor {
            height: 560px;
        }
        .draft-tip {
            margin-top: 6px;
            font-size: 12px;
            opacity: 0.55;
        }
    }

    .compose-md-toolbar {
        margin-top: 12px;
        display: flex;
        justify-content: space-between;

        .attachment {
            display: flex;
            align-items: center;
        }

        .submit-wrap {
            display: flex;
            align-items: center;
            .text-statistic {
                margin-right: 8px;
                width: 20px;
                height: 20px;
                transform: rotate(180deg);
            }
        }
    }

    .attachment-list-wrap {
        margin-top: 12px;
        .n-upload-file-info__thumbnail {
            overflow: hidden;
        }
    }

    .link-wrap {
        margin-top: 12px;
    }
    .eye-wrap {
        margin-top: 12px;
    }
}

@media (max-width: 821px) {
    .compose-md-card {
        .compose-md-editor-wrap {
            .compose-md-page-editor {
                height: 420px;
            }
        }
    }
}
</style>
