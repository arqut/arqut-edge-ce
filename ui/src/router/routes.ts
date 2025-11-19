import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: () => import('../layouts/MainLayout.vue'),
    children: [
      {
        path: '',
        redirect: '/services',
      },
      {
        path: 'services',
        name: 'services',
        component: () => import('../pages/ProxyServicesPage.vue'),
      },
    ],
  },

  // 404 Not Found
  {
    path: '/:catchAll(.*)*',
    component: () => import('../pages/ErrorNotFound.vue'),
  },
];

export default routes;
