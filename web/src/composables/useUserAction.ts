import { useRouter } from 'vue-router';
import { useDialog } from "naive-ui";
import { Api } from "../utils/request";
import { useStoreUser } from '@/store/user';
import i18n from '@/locales';

/** Conversation replies are authorized by the chat history response. */
export const canWhisperUser = () => useStoreUser().hasPermission('message.initiate');

/**
 * 私信入口统一跳转消息页会话(替代原 whisper 弹窗)
 */
export const useChatJump = () => {
    const router = useRouter();
    const goWhisper = (user: { id: number; username?: string; nickname?: string; avatar?: string }) => {
        router.push({
            name: 'messages',
            query: {
                to: user.id,
                u: user.username || '',
                n: user.nickname || '',
                a: user.avatar || '',
            },
        });
    };
    return { goWhisper };
};

export default class UserAction {

    /**
     * 用户关注/取消关注操作
     * @param dialog dialog 实例（从组件中传入）
     * @param userId 目标用户ID
     * @param userName 目标用户名（用于提示）
     * @param isFollowing 当前是否已关注（用于确定操作类型和提示文案）
     */
    static followAction(dialog: ReturnType<typeof useDialog>, userId: number, userName: string, isFollowing: boolean) {
        if (!useStoreUser().hasPermission('community.interact')) return Promise.reject(new Error(i18n.global.t('identity.accessDenied')));
        return new Promise<boolean>((resolve, reject) => {
            dialog.success({
                title: i18n.global.t('common.tip'),
                content: i18n.global.t('user.followDialogConfirm', {
                    action: isFollowing ? i18n.global.t('user.unfollowAction') : i18n.global.t('user.followAction'),
                    username: userName,
                }),
                positiveText: i18n.global.t('common.confirm'),
                negativeText: i18n.global.t('common.cancel'),
                onPositiveClick: () => {
                    if (isFollowing) {
                        Api.v1.user.post.unfollow({
                            user_id: userId,
                        })
                        .then((_res) => {
                            window.$message.success(i18n.global.t('common.operationSuccess'));
                            resolve(false);
                        })
                        .catch((_err) => {
                            reject(_err);
                        });
                    } else {
                        Api.v1.user.post.follow({
                            user_id: userId,
                        })
                        .then((_res) => {
                            window.$message.success(i18n.global.t('user.followSuccess'));
                            resolve(true);
                        })
                        .catch((_err) => {
                            reject(_err);
                        });
                    }
                },
            });
        });
    }

    /**
     * 用户私信入口已统一为路由跳转消息页会话, 见 useChatJump()
     */
}
