<template>
    <div>
        <main-nav :title="t('setting.title')" theme />
        <n-card :title="t('setting.base.title')" size="small" class="setting-card">
            <div class="base-line avatar">
                <n-avatar
                    class="avatar-img"
                    :size="80"
                    :src="userInfo.avatar"
                />
                <n-upload
                    v-if="!profile.allowPhoneBind || (
                        profile.allowPhoneBind &&
                        userInfo.phone &&
                        userInfo.phone.length > 0)
                    "
                    ref="avatarRef"
                    :action="uploadGateway"
                    :headers="{
                        Authorization: uploadToken,
                    }"
                    :data="{
                        type: uploadType,
                    }"
                    @before-upload="beforeUpload"
                    @finish="finishUpload"
                >
                    <n-button size="small">{{ t('setting.base.changeAvatar') }}</n-button>
                </n-upload>
            </div>
            <div class="base-line">
                <span class="base-label">{{ t('setting.base.nickname') }}</span>
                <div v-if="!showNicknameEdit">
                    {{ userInfo.nickname }}
                </div>
                <n-input
                    ref="inputInstRef"
                    v-show="showNicknameEdit"
                    class="nickname-input"
                    v-model:value="userInfo.nickname"
                    type="text"
                    size="small"
                    :placeholder="t('setting.base.nicknamePlaceholder')"
                    @blur="handleNicknameChange"
                    :maxlength="16"
                />
                <n-button
                    quaternary
                    round
                    type="success"
                    size="small"
                    v-if="!showNicknameEdit && (!profile.allowPhoneBind || (
                        profile.allowPhoneBind &&
                        userInfo.phone &&
                        userInfo.phone.length > 0 &&
                        userInfo.status == 1)
                    )
                    "
                    @click="handleNicknameShow"
                >
                    <template #icon>
                        <n-icon>
                            <edit />
                        </n-icon>
                    </template>
                </n-button>
            </div>
            <div class="base-line">
                <span class="base-label">{{ t('setting.base.username') }}</span> @{{
                    userInfo.username
                }}
            </div>
        </n-card>

        <n-card v-if="profile.allowPhoneBind" :title="t('setting.phone.title')" size="small" class="setting-card">
            <div
                v-if="
                    userInfo.phone &&
                    userInfo.phone.length > 0
                "
            >
                {{ userInfo.phone }}

                <n-button
                    quaternary
                    round
                    type="success"
                    v-if="!showPhoneBind && userInfo.status == 1"
                    @click="showPhoneBind = true"
                >
                    {{ t('setting.phone.rebind') }}
                </n-button>
            </div>
            <div v-else>
                <n-alert :title="t('setting.phone.alertTitle')" type="warning">
                    {{ t('setting.phone.alertContent') }}<br />
                    <a
                        class="hash-link"
                        @click="showPhoneBind = true"
                        v-if="!showPhoneBind"
                    >
                        {{ t('setting.phone.bindNow') }}
                    </a>
                </n-alert>
            </div>

            <div class="phone-bind-wrap" v-if="showPhoneBind">
                <n-form
                    ref="phoneFormRef"
                    :model="modelData"
                    :rules="bindRules"
                >
                    <n-form-item path="phone" :label="t('setting.phone.label')">
                        <n-input
                            :value="modelData.phone"
                            @update:value="(v: string) => (modelData.phone = v.trim())"
                            :placeholder="t('setting.phone.placeholder')"
                            @keydown.enter.prevent
                        />
                    </n-form-item>
                    <n-form-item path="img_captcha" :label="t('setting.phone.imgCaptchaLabel')">
                        <div class="captcha-img-wrap">
                            <n-input
                                v-model:value="modelData.imgCaptcha"
                                :placeholder="t('setting.phone.imgCaptchaPlaceholder')"
                            />
                            <div class="captcha-img">
                                <img
                                    v-if="modelData.b64s"
                                    :src="modelData.b64s"
                                    @click="loadCaptcha"
                                />
                            </div>
                        </div>
                    </n-form-item>
                    <n-form-item path="phone_captcha" :label="t('setting.phone.smsLabel')">
                        <n-input-group>
                            <n-input
                                v-model:value="modelData.phone_captcha"
                                :placeholder="t('setting.phone.smsPlaceholder')"
                            />
                            <n-button
                                type="primary"
                                ghost
                                :disabled="smsDisabled"
                                :loading="sending"
                                @click="sendPhoneCaptcha"
                            >
                                {{
                                    smsCounter > 0 && smsDisabled
                                        ? t('setting.phone.resend', { seconds: smsCounter })
                                        : t('setting.phone.send')
                                }}
                            </n-button>
                        </n-input-group>
                    </n-form-item>
                    <n-row :gutter="[0, 24]">
                        <n-col :span="24">
                            <div class="form-submit-wrap">
                                <n-button
                                    quaternary
                                    round
                                    @click="showPhoneBind = false"
                                >
                                    {{ t('common.cancel') }}
                                </n-button>
                                <n-button
                                    secondary
                                    round
                                    type="primary"
                                    :loading="binding"
                                    @click="handlePhoneBind"
                                >
                                    {{ t('setting.phone.bind') }}
                                </n-button>
                            </div>
                        </n-col>
                    </n-row>
                </n-form>
            </div>
        </n-card>

        <n-card v-if="allowActivation" :title="t('setting.activation.title')" size="small" class="setting-card">
            <div
                v-if="
                    userInfo.activation &&
                    userInfo.activation.length > 0
                "
            >
                {{ userInfo.activation }}

                <n-button
                    quaternary
                    round
                    type="success"
                    v-if="!showActivation"
                    @click="showActivation = true"
                >
                    {{ t('setting.activation.reActivate') }}
                </n-button>
            </div>
            <div v-else>
                <n-alert :title="t('setting.activation.alertTitle')" type="warning">
                    {{ t('setting.activation.alertContent') }}<br />
                    <a
                        class="hash-link"
                        @click="showActivation = true"
                        v-if="!showActivation"
                    >
                    {{ t('setting.activation.activateNow') }}
                    </a>
                </n-alert>
            </div>

            <div class="phone-bind-wrap" v-if="showActivation">
                <n-form
                    ref="activateFormRef"
                    :model="activateData"
                    :rules="activateRules"
                >
                    <n-form-item path="activate_code" :label="t('setting.activation.label')">
                        <n-input
                            :value="activateData.activate_code"
                            @update:value="(v: string) => (activateData.activate_code = v.trim())"
                            :placeholder="t('setting.activation.placeholder')"
                            @keydown.enter.prevent
                        />
                    </n-form-item>
                    <n-form-item path="img_captcha" :label="t('setting.phone.imgCaptchaLabel')">
                        <div class="captcha-img-wrap">
                            <n-input
                                v-model:value="activateData.imgCaptcha"
                                :placeholder="t('setting.phone.imgCaptchaPlaceholder')"
                            />
                            <div class="captcha-img">
                                <img
                                    v-if="activateData.b64s"
                                    :src="activateData.b64s"
                                    @click="loadCaptcha4Activate"
                                />
                            </div>
                        </div>
                    </n-form-item>
                    <n-row :gutter="[0, 24]">
                        <n-col :span="24">
                            <div class="form-submit-wrap">
                                <n-button
                                    quaternary
                                    round
                                    @click="showActivation = false"
                                >
                                    {{ t('common.cancel') }}
                                </n-button>
                                <n-button
                                    secondary
                                    round
                                    type="primary"
                                    :loading="activating"
                                    @click="handleActivation"
                                >
                                    {{ t('setting.activation.activate') }}
                                </n-button>
                            </div>
                        </n-col>
                    </n-row>
                </n-form>
            </div>
        </n-card>

        <n-card :title="t('setting.password.title')" size="small" class="setting-card">
            {{ t('setting.password.hasSet') }}
            <n-button
                quaternary
                round
                type="success"
                v-if="!showPasswordSetting"
                @click="showPasswordSetting = true"
            >
                {{ t('setting.password.reset') }}
            </n-button>
            <div class="phone-bind-wrap" v-if="showPasswordSetting">
                <n-form ref="formRef" :model="modelData" :rules="passwordRules">
                    <n-form-item path="old_password" :label="t('setting.password.oldLabel')">
                        <n-input
                            v-model:value="modelData.old_password"
                            type="password"
                            :placeholder="t('setting.password.oldPlaceholder')"
                            @keydown.enter.prevent
                        />
                    </n-form-item>
                    <n-form-item path="password" :label="t('setting.password.newLabel')">
                        <n-input
                            v-model:value="modelData.password"
                            type="password"
                            :placeholder="t('setting.password.newPlaceholder')"
                            @input="handlePasswordInput"
                            @keydown.enter.prevent
                        />
                    </n-form-item>
                    <n-form-item
                        ref="rPasswordFormItemRef"
                        first
                        path="reenteredPassword"
                        :label="t('setting.password.repeatLabel')"
                    >
                        <n-input
                            v-model:value="modelData.reenteredPassword"
                            :disabled="!modelData.password"
                            type="password"
                            :placeholder="t('setting.password.repeatPlaceholder')"
                            @keydown.enter.prevent
                        />
                    </n-form-item>
                    <n-row :gutter="[0, 24]">
                        <n-col :span="24">
                            <div class="form-submit-wrap">
                                <n-button
                                    quaternary
                                    round
                                    @click="showPasswordSetting = false"
                                >
                                    {{ t('common.cancel') }}
                                </n-button>
                                <n-button
                                    secondary
                                    round
                                    type="primary"
                                    :loading="passwordSetting"
                                    @click="handleValidateButtonClick"
                                >
                                    {{ t('setting.password.update') }}
                                </n-button>
                            </div>
                        </n-col>
                    </n-row>
                </n-form>
            </div>
        </n-card>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref, reactive, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useStoreMain } from '@/store/main';
import { Edit } from '@vicons/tabler';
import type {
  UploadInst,
  FormItemRule,
  FormItemInst,
  FormInst,
  InputInst,
} from 'naive-ui';
import { TOKEN_KEY, useStoreUser } from '@/store/user';
import { useStoreProfile } from '@/store/profile';
import { storeToRefs } from 'pinia';
import { Api } from '@/utils/request';
import { userInfo as fetchUserInfo } from '@/api/auth';

const { t } = useI18n();

const uploadGateway = import.meta.env.VITE_HOST + '/v1/attachment';
const uploadToken = 'Bearer ' + localStorage.getItem(TOKEN_KEY);
const uploadType = ref('public/avatar');
const allowActivation =
  import.meta.env.VITE_ALLOW_ACTIVATION.toLowerCase() === 'true';

const storeMain = useStoreMain();
const storeUser = useStoreUser();
const storeProfile = useStoreProfile();
const { userInfo } = storeToRefs(storeUser);
const { profile } = storeToRefs(storeProfile);

const sending = ref(false);
const binding = ref(false);
const activating = ref(false);
const avatarRef = ref<UploadInst>();
const inputInstRef = ref<InputInst>();
const showNicknameEdit = ref(false);
const passwordSetting = ref(false);
const showPasswordSetting = ref(false);
const smsDisabled = ref(false);
const smsCounter = ref(60);
const showPhoneBind = ref(false);
const showActivation = ref(false);
const phoneFormRef = ref<FormInst>();
const activateFormRef = ref<FormInst>();
const formRef = ref<FormInst>();
const rPasswordFormItemRef = ref<FormItemInst>();
const modelData = reactive({
  id: '',
  b64s: '',
  imgCaptcha: '',
  phone: '',
  phone_captcha: '',
  password: '',
  old_password: '',
  reenteredPassword: '',
});

const activateData = reactive({
  id: '',
  b64s: '',
  imgCaptcha: '',
  activate_code: '',
});

const beforeUpload = async (data: any) => {
  // 图片类型校验
  if (
    uploadType.value === 'public/avatar' &&
    !['image/png', 'image/jpg', 'image/jpeg'].includes(data.file.file?.type)
  ) {
    window.$message.warning(t('setting.avatar.formatError'));
    return false;
  }

  if (uploadType.value === 'image' && data.file.file?.size > 1048576) {
    window.$message.warning(t('setting.avatar.sizeError'));
    return false;
  }

  return true;
};

const finishUpload = ({ file, event }: any): any => {
  try {
    let data = JSON.parse(event.target?.response);

    if (data.code === 0) {
      if (uploadType.value === 'public/avatar') {
        Api.v1.user.post.avatar({
          avatar: data.data.content,
        })
          .then((res) => {
            window.$message.success(t('setting.avatar.updateSuccess'));
            avatarRef.value?.clear();

            storeUser.updateUserinfo({
              ...userInfo.value,
              avatar: data.data.content,
            });
          })
          .catch((err) => {
            console.log(err);
          });
      }
    }
  } catch (error) {
    window.$message.error(t('setting.avatar.uploadFailed'));
  }
};

const validatePasswordStartWith = (rule: FormItemRule, value: any) => {
  return (
    !!modelData.password &&
    (modelData.password as any).startsWith(value) &&
    (modelData.password as any).length >= value.length
  );
};

const validatePasswordSame = (rule: FormItemRule, value: any) => {
  return value === modelData.password;
};

const handlePasswordInput = () => {
  if (modelData.reenteredPassword) {
    rPasswordFormItemRef.value?.validate({ trigger: 'password-input' });
  }
};

const handleValidateButtonClick = (e: MouseEvent) => {
  e.preventDefault();
  formRef.value?.validate((errors) => {
    if (!errors) {
      passwordSetting.value = true;
      Api.v1.user.post.password({
        password: modelData.password,
        old_password: modelData.old_password,
      })
        .then((res) => {
          passwordSetting.value = false;
          showPasswordSetting.value = false;
          window.$message.success(t('setting.password.resetSuccess'));

          // 用户退出登录
          storeUser.userLogout();
          storeMain.triggerAuth(true);
          storeMain.triggerAuthKey('signin');
        })
        .catch((err) => {
          passwordSetting.value = false;
        });
    }
  });
};

const handlePhoneBind = (e: MouseEvent) => {
  e.preventDefault();
  phoneFormRef.value?.validate((errors) => {
    if (!errors) {
      binding.value = true;
      Api.v1.user.post.phone({
        phone: modelData.phone,
        captcha: modelData.phone_captcha,
      })
        .then((res) => {
          binding.value = false;
          showPhoneBind.value = false;
          window.$message.success(t('setting.phone.bindSuccess'));

          storeUser.updateUserinfo({
            ...userInfo.value,
            phone: modelData.phone,
          });

          modelData.id = '';
          modelData.b64s = '';
          modelData.imgCaptcha = '';
          modelData.phone = '';
          modelData.phone_captcha = '';
        })
        .catch((err) => {
          binding.value = false;
        });
    }
  });
};

const handleActivation = (e: MouseEvent) => {
  e.preventDefault();
  activateFormRef.value?.validate((errors) => {
    if (activateData.imgCaptcha === '') {
      window.$message.warning(t('setting.imgCaptchaRequired'));
      return;
    }
    sending.value = true;
    if (!errors) {
      activating.value = true;
      Api.v1.user.post.activate({
        activate_code: activateData.activate_code,
        captcha_id: activateData.id,
        imgCaptcha: activateData.imgCaptcha,
      })
        .then((res) => {
          activating.value = false;
          showActivation.value = false;
          window.$message.success(t('setting.activation.activateSuccess'));

          storeUser.updateUserinfo({
            ...userInfo.value,
            activation: activateData.activate_code,
          });

          activateData.id = '';
          activateData.b64s = '';
          activateData.imgCaptcha = '';
          activateData.activate_code = '';
        })
        .catch((err) => {
          activating.value = false;
          if (err.code === 20012) {
            loadCaptcha4Activate();
          }
        });
    }
  });
};

const loadCaptcha = () => {
  Api.v1.captcha.get._self({})
    .then((res) => {
      modelData.id = res.id;
      modelData.b64s = res.b64s;
    })
    .catch((err) => {
      console.log(err);
    });
};

const loadCaptcha4Activate = () => {
  Api.v1.captcha.get._self({})
    .then((res) => {
      activateData.id = res.id;
      activateData.b64s = res.b64s;
    })
    .catch((err) => {
      console.log(err);
    });
};

const handleNicknameChange = () => {
  const submitted = userInfo.value.nickname || '';
  Api.v1.user.post.nickname({
    nickname: submitted,
  })
    .then(async (_res) => {
      showNicknameEdit.value = false;
      // 昵称可能需要审核: 重新拉取用户信息以恢复服务端权威昵称
      // 返回昵称==提交值说明立即生效(管理角色) 否则处于待审核状态
      try {
        const fresh = await fetchUserInfo();
        storeUser.updateUserinfo(fresh);
        if ((fresh.nickname || '') === submitted) {
          window.$message.success(t('setting.nickname.changeSuccess'));
        } else {
          window.$message.info(t('setting.nickname.auditPending'));
        }
      } catch (_err) {
        window.$message.success(t('setting.nickname.submitted'));
      }
    })
    .catch((err) => {
      showNicknameEdit.value = true;
    });
};

const sendPhoneCaptcha = () => {
  if (smsCounter.value > 0 && smsDisabled.value) {
    return;
  }
  if (modelData.imgCaptcha === '') {
    window.$message.warning(t('setting.imgCaptchaRequired'));
    return;
  }
  sending.value = true;
  Api.v1.captcha.post._self({
    phone: modelData.phone,
    img_captcha: modelData.imgCaptcha,
    img_captcha_id: modelData.id,
  })
    .then((res) => {
      smsDisabled.value = true;
      sending.value = false;
      window.$message.success(t('setting.sendSuccess'));

      let s = setInterval(() => {
        smsCounter.value--;
        if (smsCounter.value === 0) {
          clearInterval(s);
          smsCounter.value = 60;
          smsDisabled.value = false;
        }
      }, 1000);
    })
    .catch((err) => {
      sending.value = false;
      if (err.code === 20012) {
        loadCaptcha();
      }
      console.log(err);
    });
};

const bindRules = computed(() => ({
  phone: [
    {
      required: true,
      message: t('setting.rule.phoneRequired'),
      trigger: ['input'],
      validator: (rule: FormItemRule, value: any) => {
        return /^[1]+[3-9]{1}\d{9}$/.test(value);
      },
    },
  ],
  phone_captcha: [
    {
      required: true,
      message: t('setting.rule.smsRequired'),
    },
  ],
}));

const activateRules = computed(() => ({
  activate_code: [
    {
      required: true,
      message: t('setting.rule.activationRequired'),
      trigger: ['input'],
      validator: (rule: FormItemRule, value: any) => {
        return /\d{6}$/.test(value);
      },
    },
  ],
}));

const passwordRules = computed(() => ({
  password: [
    {
      required: true,
      message: t('setting.rule.newPasswordRequired'),
    },
  ],
  old_password: [
    {
      required: true,
      message: t('setting.rule.oldPasswordRequired'),
    },
  ],
  reenteredPassword: [
    {
      required: true,
      message: t('setting.rule.repeatRequired'),
      trigger: ['input', 'blur'],
    },
    {
      validator: validatePasswordStartWith,
      message: t('setting.rule.passwordMismatch'),
      trigger: 'input',
    },
    {
      validator: validatePasswordSame,
      message: t('setting.rule.passwordMismatch'),
      trigger: ['blur', 'password-input'],
    },
  ],
}));

const handleNicknameShow = () => {
  showNicknameEdit.value = true;
  setTimeout(() => {
    inputInstRef.value?.focus();
  }, 30);
};
onMounted(() => {
  if (userInfo.value.id === 0) {
    storeMain.triggerAuth(true);
    storeMain.triggerAuthKey('signin');
  }
  loadCaptcha();
  loadCaptcha4Activate();
});
</script>

<style lang="less" scoped>
.setting-card {
    margin-top: -1px;
    border-radius: 0;
    .form-submit-wrap {
        display: flex;
        justify-content: flex-end;
    }

    .base-line {
        line-height: 2;
        display: flex;
        align-items: center;
        .base-label {
            opacity: 0.75;
            margin-right: 12px;
        }

        .nickname-input {
            margin-right: 10px;
            width: 120px;
        }
    }

    .avatar {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        margin-bottom: 20px;
        .avatar-img {
            margin-bottom: 10px;
        }
    }

    .hash-link {
        margin-left: 12px;
    }

    .phone-bind-wrap {
        margin-top: 20px;
        .captcha-img-wrap {
            width: 100%;
            display: flex;
            align-items: center;
        }
        .captcha-img {
            width: 125px;
            height: 34px;
            border-radius: 3px;
            margin-left: 10px;
            overflow: hidden;
            cursor: pointer;
            img {
                width: 100%;
                height: 100%;
            }
        }
    }
}
.dark {
    .setting-card {
        background-color: rgba(16, 16, 20, 0.75);
    }
}
</style>
