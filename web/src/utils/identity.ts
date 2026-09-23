/**
 * @file 用户身份展示辅助
 */

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
