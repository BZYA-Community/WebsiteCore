declare namespace Api {

    namespace Admin {

        interface Api {
            post: {
                user: {
                    /** 管理·用户禁言/解禁 */
                    status: (params: NetParams.UserStatusReq) => Promise<NetReq.UserChangeStatus>;
                    /** 管理·变更用户角色 */
                    role: (params: NetParams.UserRoleChangeReq) => Promise<NetReq.UserRoleChangeResp>;
                    /** 管理·软删除用户 */
                    delete: (params: NetParams.UserDeleteReq) => Promise<NetReq.UserDeleteResp>;
                },
                site: {
                    /** 管理·更新系统配置 */
                    profile: (params: NetParams.SiteProfileReq) => Promise<NetReq.SiteProfileResp>;
                },
                settings: {
                    /** 管理·保存通用配置 */
                    save: (params: NetParams.SettingsSaveReq) => Promise<NetReq.SettingsSaveResp>;
                },
                audit: {
                    /** 审核·通过/拒绝/删除帖子 */
                    post: (params: NetParams.AuditPostReq) => Promise<NetReq.AuditPostResp>;
                    /** 审核·通过/拒绝评论或回复 */
                    comment: (params: NetParams.AuditCommentReq) => Promise<NetReq.AuditCommentResp>;
                    /** 审核·通过/拒绝昵称变更 */
                    nickname: (params: NetParams.AuditNicknameReq) => Promise<NetReq.AuditNicknameResp>;
                }
            },
            get: {
                site: {
                    /** 获取站点状态信息 */
                    status: () => Promise<NetReq.SiteInfoResp>;
                    /** 管理·获取系统配置 */
                    profile: () => Promise<NetReq.SiteProfileResp>;
                },
                settings: {
                    /** 管理·获取通用配置 schema */
                    schema: () => Promise<NetReq.SettingsSchemaResp>;
                    /** 管理·获取通用配置当前值 */
                    values: () => Promise<NetReq.SettingsValuesResp>;
                },
                user: {
                    /** 管理·搜索用户列表 */
                    list: (params: NetParams.UserListReq) => Promise<NetReq.UserListResp>;
                    /** 管理·用户详情(完整手机号) */
                    detail: (params: NetParams.UserDetailReq) => Promise<NetReq.UserDetailResp>;
                    /** 管理·角色变更记录 */
                    role: {
                        logs: (params: NetParams.PageReq) => Promise<NetReq.UserRoleLogsResp>;
                    }
                },
                audit: {
                    /** 审核·帖子队列 */
                    posts: (params: NetParams.AuditPostsReq) => Promise<NetReq.AuditPostsResp>;
                    /** 审核·评论/回复队列 */
                    comments: (params: NetParams.AuditCommentsReq) => Promise<NetReq.AuditCommentsResp>;
                    /** 审核·昵称变更队列 */
                    nicknames: (params: NetParams.PageReq) => Promise<NetReq.AuditNicknamesResp>;
                    /** 审核·操作日志 */
                    logs: (params: NetParams.PageReq) => Promise<NetReq.AuditLogsResp>;
                }
            }
        }

        namespace NetParams {

            interface UserStatusReq {
                id: number;
                status: number;
            }

            interface UserRoleChangeReq {
                user_id: number;
                role: 'mentor' | 'auditor' | 'admin' | 'operator';
                action: 'add' | 'remove';
            }

            interface UserDeleteReq {
                id: number;
            }

            interface UserListReq {
                keyword?: string;
                page: number;
                page_size: number;
            }

            interface UserDetailReq {
                id: number;
            }

            interface PageReq {
                page: number;
                page_size: number;
            }

            interface AuditPostsReq {
                /** 0待审核 1已通过 2未通过 */
                status: number;
                page: number;
                page_size: number;
            }

            interface AuditPostReq {
                post_id: number;
                action: 'approve' | 'reject';
                reason?: string;
            }

            interface AuditCommentsReq {
                /** 0待审核 1已通过 2未通过 */
                status: number;
                page: number;
                page_size: number;
            }

            interface AuditCommentReq {
                id: number;
                /** 0评论 1回复 */
                comment_type: 0 | 1;
                action: 'approve' | 'reject';
                reason?: string;
            }

            interface AuditNicknameReq {
                user_id: number;
                action: 'approve' | 'reject';
                reason?: string;
            }

            interface SiteProfileReq {
                use_friendship: boolean;
                enable_trends_bar: boolean;
                allow_tweet_attachment: boolean;
                allow_tweet_video: boolean;
                default_tweet_max_length: number;
                tweet_web_ellipsis_size: number;
                tweet_mobile_ellipsis_size: number;
                default_tweet_visibility: 'public' | 'following' | 'friend' | 'private';
                default_msg_loop_interval: number;
                copyright_top: string;
                copyright_left: string;
                copyright_left_link: string;
                copyright_right: string;
                copyright_right_link: string;
            }

            interface SettingValueInput {
                key: string;
                value: string | number | boolean;
            }

            interface SettingsSaveReq {
                items: SettingValueInput[];
            }

        }

        namespace NetReq {
            interface UserChangeStatus {}

            interface UserRoleChangeResp {}

            interface UserDeleteResp {}

            interface AuditPostResp {}

            interface AuditCommentResp {}

            interface AuditNicknameResp {}

            interface Pager {
                page: number;
                page_size: number;
                total_rows: number;
            }

            interface UserItem {
                id: number;
                nickname: string;
                username: string;
                phone: string;
                roles: string[];
                identity: string;
                status: 1 | 2;
                is_admin: boolean;
                created_on: number;
            }

            interface UserListResp {
                list: UserItem[];
                pager: Pager;
            }

            interface UserDetailResp extends UserItem {}

            interface UserRoleLogItem {
                id: number;
                user_id: number;
                username: string;
                operator_id: number;
                operator_name: string;
                old_roles: string;
                new_roles: string;
                action: string;
                created_on: number;
            }

            interface UserRoleLogsResp {
                list: UserRoleLogItem[];
                pager: Pager;
            }

            interface AuditPostItem {
                id: number;
                user: {
                    id: number;
                    nickname: string;
                    username: string;
                };
                contents: {
                    id: number;
                    content: string;
                    /** 1标题 2文字 3图片 4视频 5音频 6链接 7附件 8收费附件 */
                    type: number;
                    sort: number;
                }[];
                /** 0私密 50好友可见 60关注可见 90公开 */
                visibility: number;
                created_on: number;
                /** 0待审核 1已通过 2未通过 */
                audit_status: 0 | 1 | 2;
            }

            interface AuditPostsResp {
                list: AuditPostItem[];
                pager: Pager;
            }

            interface AuditCommentItem {
                id: number;
                /** 0评论 1回复 */
                comment_type: 0 | 1;
                post_id: number;
                /** 回复所属评论ID(评论自身为0) */
                comment_id: number;
                user?: {
                    id: number;
                    nickname: string;
                    username: string;
                };
                content: string;
                /** 0待审核 1已通过 2未通过 */
                audit_status: 0 | 1 | 2;
                created_on: number;
            }

            interface AuditCommentsResp {
                list: AuditCommentItem[];
                pager: Pager;
            }

            interface AuditNicknameItem {
                user_id: number;
                username: string;
                /** 当前昵称 */
                nickname: string;
                /** 待审核昵称 */
                pending_nickname: string;
                created_on: number;
            }

            interface AuditNicknamesResp {
                list: AuditNicknameItem[];
                pager: Pager;
            }

            interface AuditLogItem {
                id: number;
                post_id: number;
                operator_id: number;
                operator_name: string;
                action: 'approve' | 'reject' | 'delete' | string;
                old_status: number;
                new_status: number;
                reason: string;
                created_on: number;
            }

            interface AuditLogsResp {
                list: AuditLogItem[];
                pager: Pager;
            }

            interface SettingOption {
                label: string;
                value: string | number | boolean;
            }

            interface SettingsSchemaItem {
                key: string;
                group: string;
                section: string;
                type: 'bool' | 'int' | 'float' | 'string';
                label: string;
                description: string;
                apply_mode: 'live' | 'restart_required' | 'bootstrap_only';
                secret: boolean;
                readonly: boolean;
                active: boolean;
                bootstrap_value?: string | number | boolean;
                bootstrap_configured?: boolean;
                options?: SettingOption[];
            }

            interface SettingsSchemaResp {
                items: SettingsSchemaItem[];
            }

            interface SettingsValueItem {
                key: string;
                value?: string | number | boolean;
                effective_value?: string | number | boolean;
                source: 'bootstrap' | 'override' | string;
                pending_restart: boolean;
                configured?: boolean;
                active: boolean;
            }

            interface SettingsValuesResp {
                items: SettingsValueItem[];
                has_pending_restart: boolean;
            }

            interface SettingsSaveResp {
                items: SettingsValueItem[];
                updated_keys: string[];
                has_pending_restart: boolean;
            }

            interface SiteInfoResp {
                register_user_count: number;
                online_user_count: number;
                history_max_online: number;
                server_up_time: number;
            }

            interface SiteProfileResp {
                use_friendship: boolean;
                enable_trends_bar: boolean;
                allow_tweet_attachment: boolean;
                allow_tweet_video: boolean;
                allow_user_register: boolean;
                allow_phone_bind: boolean;
                default_tweet_max_length: number;
                tweet_web_ellipsis_size: number;
                tweet_mobile_ellipsis_size: number;
                default_tweet_visibility: 'public' | 'following' | 'friend' | 'private';
                default_msg_loop_interval: number;
                copyright_top: string;
                copyright_left: string;
                copyright_left_link: string;
                copyright_right: string;
                copyright_right_link: string;
                readonly_fields: string[];
            }
        }

    }

}
