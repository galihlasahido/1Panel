import { Layout } from '@/routers/constant';

const securityRouter = {
    sort: 12,
    path: '/security',
    name: 'Security-Menu',
    component: Layout,
    redirect: '/security',
    meta: {
        title: 'menu.security',
        icon: 'p-safe',
    },
    children: [
        {
            path: '/security',
            name: 'Security',
            component: () => import('@/views/security/index.vue'),
            meta: {
                requiresAuth: true,
                title: 'menu.security',
                activeMenu: '/security',
            },
        },
    ],
};

export default securityRouter;
