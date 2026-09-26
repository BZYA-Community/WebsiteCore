/**
 * vue-i18n 类型推导: 以 zh-CN 语言包为 schema 源,
 * t() 获得 key 自动补全与类型检查。
 * 新增 namespace JSON 后需在此登记。
 */
import type common from '../locales/zh-CN/common.json';
import type nav from '../locales/zh-CN/nav.json';
import type sidebar from '../locales/zh-CN/sidebar.json';
import type auth from '../locales/zh-CN/auth.json';
import type post from '../locales/zh-CN/post.json';
import type comment from '../locales/zh-CN/comment.json';
import type compose from '../locales/zh-CN/compose.json';
import type message from '../locales/zh-CN/message.json';
import type user from '../locales/zh-CN/user.json';
import type setting from '../locales/zh-CN/setting.json';
import type course from '../locales/zh-CN/course.json';
import type admin from '../locales/zh-CN/admin.json';
import type adminAudit from '../locales/zh-CN/adminAudit.json';
import type adminUsers from '../locales/zh-CN/adminUsers.json';
import type adminSettings from '../locales/zh-CN/adminSettings.json';
import type errors from '../locales/zh-CN/errors.json';

declare module 'vue-i18n' {
  interface DefineLocaleMessage {
    common: typeof common;
    nav: typeof nav;
    sidebar: typeof sidebar;
    auth: typeof auth;
    post: typeof post;
    comment: typeof comment;
    compose: typeof compose;
    message: typeof message;
    user: typeof user;
    setting: typeof setting;
    course: typeof course;
    admin: typeof admin;
    adminAudit: typeof adminAudit;
    adminUsers: typeof adminUsers;
    adminSettings: typeof adminSettings;
    errors: typeof errors;
  }
}

export {};
