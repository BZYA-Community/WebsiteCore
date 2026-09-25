<template>
    <div>
        <div class="compose-wrap" v-if="userInfo.id > 0">
            <div class="compose-line">
                <div class="compose-user">
                    <n-avatar
                        round
                        :size="30"
                        :src="userInfo.avatar"
                    />
                </div>
                <n-mention
                    type="textarea"
                    size="large"
                    autosize
                    :bordered="false"
                    :options="optionsRef"
                    :prefix="['@']"
                    :loading="loading"
                    :value="content"
                    @update:value="changeContent"
                    @search="handleSearch"
                    @focus="focusComment"
                    placeholder="快来评论两句吧..."
                />
            </div>

            <n-upload
                v-if="showBtn"
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
                <div class="compose-line compose-options">
                    <div class="attachment">
                        <n-upload-trigger #="{ handleClick }" abstract>
                            <n-button
                                :disabled="fileQueue.length === 9"
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

                        <n-tooltip trigger="hover" placement="bottom">
                            <template #trigger>
                                <n-progress
                                    class="text-statistic"
                                    type="circle"
                                    :show-indicator="false"
                                    status="success"
                                    :stroke-width="10"
                                    :percentage="(content.length / defaultCommentMaxLength) * 100"
                                />
                            </template>
                            {{ content.length }} / {{ defaultCommentMaxLength }}
                        </n-tooltip>
                    </div>

                    <div class="submit-wrap">
                        <n-button
                            quaternary
                            round
                            type="tertiary"
                            class="cancel-btn"
                            size="small"
                            @click="cancelComment"
                        >
                            取消
                        </n-button>
                        <n-button
                            :loading="submitting"
                            @click="submitPost"
                            type="primary"
                            secondary
                            size="small"
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
        </div>

        <div class="compose-wrap" v-else>
            <div class="login-wrap">
                <span class="login-banner"> 登录后，精彩更多</span>
            </div>
            <div class="login-wrap">
                <n-button
                    strong
                    secondary
                    round
                    type="primary"
                    @click="triggerAuth('signin')"
                >
                    登录
                </n-button>
                <n-button
                    v-if="allowUserRegister"
                    strong
                    secondary
                    round
                    type="info"
                    @click="triggerAuth('signup')"
                >
                    注册
                </n-button>
            </div>
        </div>
    </div>
</template>


<script setup lang="ts">
import { computed, ref } from 'vue';
import { useStoreMain } from '@/store/main';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { debounce } from 'lodash';
import { ImageOutline } from '@vicons/ionicons5';
import { createCourseComment } from '@/api/course';
import { parsePostTag } from '@/utils/content';
import type { MentionOption, UploadFileInfo, UploadInst } from 'naive-ui';
import { storeToRefs } from 'pinia';
import { Api } from '@/utils/request';

const emit = defineEmits<{
  (e: 'post-success'): void;
}>();
const props = withDefaults(
  defineProps<{
    courseId: number;
  }>(),
  {
    courseId: 0,
  },
);

const storeMain = useStoreMain();
const storeUser = useStoreUser();
const { userInfo } = storeToRefs(storeUser);

const optionsRef = ref<MentionOption[]>([]);
const showBtn = ref(false);
const loading = ref(false);
const submitting = ref(false);
const content = ref('');
const uploadRef = ref<UploadInst>();
const uploadType = ref('public/image');
const fileQueue = ref<UploadFileInfo[]>([]);
const imageContents = ref<{ id: string; content: string }[]>([]);
const allowUserRegister = ref(
  import.meta.env.VITE_ALLOW_USER_REGISTER.toLowerCase() === 'true',
);
const defaultCommentMaxLength = Number(
  import.meta.env.VITE_DEFAULT_COMMENT_MAX_LENGTH,
);
const uploadGateway = import.meta.env.VITE_HOST + '/v1/attachment';

const uploadToken = computed(() => {
  return 'Bearer ' + localStorage.getItem(TOKEN_KEY);
});
// 加载at用户列表
const loadSuggestionUsers = debounce((k) => {
  Api.v1.suggest.get
    .users({
      k,
    })
    .then((res) => {
      let options: MentionOption[] = [];
      res.suggest.map((i) => {
        options.push({
          label: i,
          value: i,
        });
      });
      optionsRef.value = options;
      loading.value = false;
    })
    .catch(() => {
      loading.value = false;
    });
}, 200);
const handleSearch = (k: string, prefix: string) => {
  if (loading.value) {
    return;
  }
  loading.value = true;
  if (prefix === '@') {
    loadSuggestionUsers(k);
  }
};
const changeContent = (v: string) => {
  if (v.length > defaultCommentMaxLength) {
    content.value = v.substring(0, defaultCommentMaxLength);
  } else {
    content.value = v;
  }
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
    !['image/png', 'image/jpg', 'image/jpeg', 'image/gif'].includes(
      (data.file as any).file?.type,
    )
  ) {
    window.$message.warning('图片仅允许 png/jpg/gif 格式');
    return false;
  }

  if (
    uploadType.value === 'image' &&
    (data.file as any).file?.size > 10485760
  ) {
    window.$message.warning('图片大小不能超过10MB');
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
        });
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
};

const focusComment = () => {
  showBtn.value = true;
};
const cancelComment = () => {
  showBtn.value = false;
  // 置空
  uploadRef.value?.clear();
  fileQueue.value = [];
  content.value = '';
  imageContents.value = [];
};

// 发布评论
const submitPost = () => {
  if (content.value.trim().length === 0) {
    window.$message.warning('请输入内容哦');
    return;
  }

  // 解析用户at
  let { users } = parsePostTag(content.value);

  const contents = [];
  let sort = 100;

  contents.push({
    content: content.value,
    type: 2, // 文字
    sort,
  });
  imageContents.value.map((img) => {
    sort++;
    contents.push({
      content: img.content,
      type: 3, // 图片
      sort,
    });
  });

  submitting.value = true;
  createCourseComment({
    contents,
    course_id: props.courseId,
    users: Array.from(new Set(users)),
  })
    .then((res) => {
      if (res.audit_status === 0) {
        window.$message.success('评论已提交，审核通过后对外可见');
      } else {
        window.$message.success('发布成功');
      }
      submitting.value = false;
      emit('post-success');

      // 置空
      cancelComment();
    })
    .catch(() => {
      submitting.value = false;
    });
};
const triggerAuth = (key: string) => {
  storeMain.triggerAuth(true);
  storeMain.triggerAuthKey(key);
};
</script>

<style lang="less" scoped>
.compose-wrap {
    width: 100%;
    padding: 16px;
    box-sizing: border-box;

    .compose-line {
        display: flex;
        flex-direction: row;

        .compose-user {
            width: 42px;
            height: 42px;
            display: flex;
            align-items: center;
        }

        &.compose-options {
            margin-top: 6px;
            padding-left: 42px;
            display: flex;
            justify-content: space-between;

            .submit-wrap {
                display: flex;
                align-items: center;

                .cancel-btn {
                    margin-right: 8px;
                }
            }
        }
    }
    .login-only-wrap {
        display: flex;
        justify-content: center;
        width: 100%;
        button {
            margin: 0 4px;
            width: 50%
        }
    }
    .login-wrap {
        display: flex;
        justify-content: center;
        width: 100%;
        .login-banner {
            margin-bottom: 12px;
            opacity: 0.8;
        }
        button {
            margin: 0 4px;
        }
    }
}
.attachment {
    display: flex;
    align-items: center;
    .text-statistic {
        margin-left: 8px;
        width: 18px;
        height: 18px;
        transform: rotate(180deg);
    }
}
.attachment-list-wrap {
    margin-top: 12px;
    margin-left: 42px;
    .n-upload-file-info__thumbnail {
        overflow: hidden;
    }
}
.dark {
    .compose-wrap {
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>
