/**
 * @file 用户身份展示辅助
 */

import i18n from '@/locales';

export type IdentityTagType =
    | 'error'
    | 'warning'
    | 'success'
    | 'info'
    | 'default';

/** 身份徽章颜色: 运维/管理员=error 审核=warning 导师=success 道友=info 游客=default */
export const identityTagType = (identity?: string): IdentityTagType => {
    switch (identity) {
        case '运维':
        case '管理员':
            return 'error';
        case '审核':
            return 'warning';
        case '导师':
            return 'success';
        case '道友':
            return 'info';
        default:
            return 'default';
    }
};

/** 是否展示身份徽章(游客不展示) */
export const showIdentityBadge = (identity?: string) =>
    !!identity && identity !== '游客';

/** 将后端协议身份值映射为 i18n 展示标签, 未匹配时回退原始字符串 */
export const identityLabel = (identity?: string): string => {
    switch (identity) {
        case '运维':
            return i18n.global.t('user.identity.operator');
        case '管理员':
            return i18n.global.t('user.identity.admin');
        case '审核':
            return i18n.global.t('user.identity.auditor');
        case '导师':
            return i18n.global.t('user.identity.mentor');
        case '道友':
            return i18n.global.t('user.identity.fellow');
        case '游客':
            return i18n.global.t('user.identity.guest');
        default:
            return identity ?? '';
    }
};
