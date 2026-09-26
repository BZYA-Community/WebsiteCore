import { createRouter, createWebHashHistory } from 'vue-router';
import { checkRouteAuth } from './guard';

const routes = [
  {
    path: '/',
    name: 'home',
    meta: {
      title: '广场',
      keepAlive: true,
    },
    component: () => import('@/views/Home.vue'),
  },
  {
    path: '/post',
    name: 'post',
    meta: {
      title: '泡泡详情',
    },
    component: () => import('@/views/Post.vue'),
  },
  {
    path: '/compose-md',
    name: 'compose-md',
    meta: {
      title: '发布长文',
      requiresAuth: true,
    },
    component: () => import('@/views/ComposeMd.vue'),
  },
  {
    path: '/courses',
    name: 'courses',
    meta: {
      title: '课程',
    },
    component: () => import('@/views/Courses.vue'),
  },
  {
    path: '/course',
    name: 'course',
    meta: {
      title: '课程详情',
    },
    component: () => import('@/views/CourseDetail.vue'),
  },
  {
    path: '/topic',
    name: 'topic',
    meta: {
      title: '话题',
    },
    component: () => import('@/views/Topic.vue'),
  },
  {
    path: '/profile',
    name: 'profile',
    meta: {
      title: '主页',
    },
    component: () => import('@/views/Profile.vue'),
  },
  {
    path: '/u',
    name: 'user',
    meta: {
      title: '用户详情',
    },
    component: () => import('@/views/User.vue'),
  },
  {
    path: '/messages',
    name: 'messages',
    meta: {
      title: '消息',
    },
    component: () => import('@/views/Chat.vue'),
  },
  {
    path: '/collection',
    name: 'collection',
    meta: {
      title: '收藏',
    },
    component: () => import('@/views/Collection.vue'),
  },
  {
    path: '/following',
    name: 'following',
    meta: {
      title: '关注',
    },
    component: () => import('@/views/Following.vue'),
  },
  {
    path: '/setting',
    name: 'setting',
    meta: {
      title: '设置',
      requiresAuth: true,
    },
    component: () => import('@/views/Setting.vue'),
  },
  {
    path: '/admin/settings',
    name: 'admin-settings',
    meta: {
      title: '系统配置',
      requiresAuth: true,
      requiresAdmin: true,
    },
    component: () => import('@/views/AdminSettings.vue'),
  },
  {
    path: '/admin/users',
    name: 'admin-users',
    meta: {
      title: '用户管理',
      requiresAuth: true,
      requiresAdmin: true,
    },
    component: () => import('@/views/AdminUsers.vue'),
  },
  {
    path: '/admin/audit',
    name: 'admin-audit',
    meta: {
      title: '审核队列',
      requiresAuth: true,
      requiresAdmin: true,
      // 审核队列对管理员与审核员（auditor）开放
      adminRoles: ['auditor'],
    },
    component: () => import('@/views/AdminAudit.vue'),
  },
  {
    path: '/404',
    name: '404',
    meta: {
      title: '404',
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

router.beforeEach(async (to) => {
  document.title = `${to.meta.title} | 泡泡 - 一个清新文艺的微社区`;

  // 路由级鉴权：只对声明了 requiresAuth / requiresAdmin 的路由生效
  if (to.meta.requiresAuth || to.meta.requiresAdmin) {
    const redirect = await checkRouteAuth(to.meta);
    if (redirect) return redirect;
  }

  return true;
});

export default router;
