import { createRouter, createWebHashHistory } from 'vue-router';
import { watch } from 'vue';
import i18n from '@/locales';
import { useStoreUser } from '@/store/user';
import { useStoreMain } from '@/store/main';

const routes = [
  {
    path: '/',
    name: 'home',
    meta: {
      titleKey: 'nav.home',
      keepAlive: true,
    },
    component: () => import('@/views/Home.vue'),
  },
  {
    path: '/post',
    name: 'post',
    meta: {
      titleKey: 'nav.postDetail',
    },
    component: () => import('@/views/Post.vue'),
  },
  {
    path: '/compose-md',
    name: 'compose-md',
    meta: {
      titleKey: 'nav.composeMd',
    },
    component: () => import('@/views/ComposeMd.vue'),
  },
  {
    path: '/courses',
    name: 'courses',
    meta: {
      titleKey: 'nav.courses',
    },
    component: () => import('@/views/Courses.vue'),
  },
  {
    path: '/course',
    name: 'course',
    meta: {
      titleKey: 'nav.courseDetail',
    },
    component: () => import('@/views/CourseDetail.vue'),
  },
  {
    path: '/topic',
    name: 'topic',
    meta: {
      titleKey: 'nav.topic',
    },
    component: () => import('@/views/Topic.vue'),
  },
  {
    path: '/profile',
    name: 'profile',
    meta: {
      titleKey: 'nav.profile',
    },
    component: () => import('@/views/Profile.vue'),
  },
  {
    path: '/u',
    name: 'user',
    meta: {
      titleKey: 'nav.userDetail',
    },
    component: () => import('@/views/User.vue'),
  },
  {
    path: '/messages',
    name: 'messages',
    meta: {
      titleKey: 'nav.messages',
    },
    component: () => import('@/views/Chat.vue'),
  },
  {
    path: '/collection',
    name: 'collection',
    meta: {
      titleKey: 'nav.collection',
    },
    component: () => import('@/views/Collection.vue'),
  },
  {
    path: '/following',
    name: 'following',
    meta: {
      titleKey: 'nav.following',
    },
    component: () => import('@/views/Following.vue'),
  },
  {
    path: '/setting',
    name: 'setting',
    meta: {
      titleKey: 'nav.setting',
    },
    component: () => import('@/views/Setting.vue'),
  },
  {
    path: '/admin/settings',
    name: 'admin-settings',
    meta: {
      titleKey: 'nav.adminSettings',
    },
    component: () => import('@/views/AdminSettings.vue'),
  },
  {
    path: '/admin/users',
    name: 'admin-users',
    meta: {
      titleKey: 'nav.adminUsers',
    },
    component: () => import('@/views/AdminUsers.vue'),
  },
  {
    path: '/admin/audit',
    name: 'admin-audit',
    meta: {
      titleKey: 'nav.adminAudit',
    },
    component: () => import('@/views/AdminAudit.vue'),
  },
  {
    path: '/404',
    name: '404',
    meta: {
      titleKey: 'nav.pageNotFound',
    },
    component: () => import('@/views/404.vue'),
  },
  {
    path: '/:pathMatch(.*)',
    redirect: '/404',
  },
];

const router = createRouter({
  history: createWebHashHistory(),
  routes,
});

function updateDocumentTitle(titleKey?: unknown) {
  const title = titleKey ? i18n.global.t(titleKey as string) : '';
  document.title = title
    ? `${title} | ${i18n.global.t('common.siteName')}`
    : i18n.global.t('common.siteName');
}

export const routePermissions: Record<string, string[]> = {
  home: ['post.view'], post: ['post.view'], topic: ['post.view'], user: ['post.view'],
  'compose-md': ['post.create'], courses: ['course.catalog'], course: ['course.view'],
  collection: ['community.interact'], following: ['community.interact'],
  messages: ['profile.edit'],
  'admin-settings': ['site.manage'], 'admin-users': ['user.manage', 'identity.manage'],
  'admin-audit': ['content.review'],
};

router.beforeEach(async (to) => {
  updateDocumentTitle(to.meta.titleKey);
  if (to.name === '404') return true;
  const user = useStoreUser();
  try {
    await user.loadSession();
  } catch {
    return { name: '404' };
  }
  if ((String(to.name).startsWith('admin-') || ['profile', 'setting', 'messages', 'collection', 'following', 'compose-md'].includes(String(to.name))) && !user.userLogined) {
    useStoreMain().triggerAuth(true);
    useStoreMain().triggerAuthKey('signin');
    return { name: 'home' };
  }
  const required = routePermissions[String(to.name)];
  return !required || user.hasAnyPermission(required) ? true : { name: '404' };
});

// 语言切换时刷新当前路由标题
watch(
  () => i18n.global.locale.value,
  () => updateDocumentTitle(router.currentRoute.value.meta.titleKey),
);

export default router;
