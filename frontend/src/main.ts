import { createApp } from 'vue';
import App from './App.vue';

import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import * as Icons from '@element-plus/icons-vue';

import '@/styles/index.scss';
import '@/styles/common.scss';
import '@/assets/iconfont/iconfont.css';
import '@/assets/iconfont/iconfont.js';
import '@/styles/style.css';
import { loadXpackStyles } from '@/extensions/theme';

loadXpackStyles();

import router from '@/routers/index';
import i18n, { ensureFallbackLocale, loadLocaleMessages } from '@/lang/index';
import pinia from '@/store/index';
import SvgIcon from './components/svg-icon/svg-icon.vue';
import Components from '@/components';

import directives from '@/directives/index';

const bootstrap = async () => {
    const currentLocale = i18n.global.locale.value;

    await Promise.all([loadLocaleMessages(currentLocale), ensureFallbackLocale()]);

    const app = createApp(App);
    app.component('SvgIcon', SvgIcon);
    app.use(ElementPlus);

    Object.keys(Icons).forEach((key) => {
        app.component(key, Icons[key as keyof typeof Icons]);
    });

    app.use(router);
    app.use(i18n);
    app.use(pinia);
    app.use(Components);
    app.use(directives);

    app.mount('#app');
};

// The axios interceptor already handles auth failures (redirect to
// login) and stale-chunk errors (reload), but it still rejects the
// promise so callers can branch. Most callers don't .catch(), so those
// already-handled rejections spam the console as "Uncaught (in
// promise)". Suppress only those known-handled shapes — real bugs still
// surface.
const AUTH_CODES = new Set([313, 401, 403]);
const isHandledRejection = (reason: any): boolean => {
    if (!reason) return false;
    const status = reason?.response?.status ?? reason?.status;
    if (typeof status === 'number' && AUTH_CODES.has(status)) return true;
    if (typeof reason.code === 'number' && AUTH_CODES.has(reason.code)) return true;
    const msg = String(reason?.message || reason?.response?.data?.message || '').toLowerCase();
    return (
        msg.includes('session expired') ||
        msg.includes('not logged in') ||
        msg.includes('csrf token invalid') ||
        msg.includes('dynamically imported module')
    );
};
window.addEventListener('unhandledrejection', (event) => {
    if (isHandledRejection(event.reason)) {
        event.preventDefault();
    }
});

bootstrap();
