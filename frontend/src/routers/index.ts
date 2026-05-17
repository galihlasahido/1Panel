import router from '@/routers/router';
import NProgress from '@/config/nprogress';
import { GlobalStore } from '@/store';
import { AxiosCanceler } from '@/api/helper/axios-cancel';
import { loadRbacPerms, isPathAllowed, firstAllowedPath } from '@/utils/rbac';

const axiosCanceler = new AxiosCanceler();

let isRedirecting = false;

router.beforeEach(async (to, from, next) => {
    NProgress.start();
    axiosCanceler.removeAllPending();
    const globalStore = GlobalStore();
    const isPublicRoute = to.name === 'entrance' || to.matched.some((record) => record.meta.requiresAuth === false);
    if (!isPublicRoute && !globalStore.isLogin) {
        next({
            name: 'entrance',
            params: to.params,
        });
        NProgress.done();
        return;
    }
    if (to.name === 'entrance' && globalStore.isLogin) {
        if (to.params.code === globalStore.entrance) {
            next({
                name: 'home',
            });
            NProgress.done();
            return;
        }
        next({ name: '404' });
        NProgress.done();
        return;
    }

    // RBAC route enforcement: a restricted sub-user must not reach a
    // page outside their allowed menus even by typing the URL (the
    // sidebar filter alone doesn't stop direct navigation). Fails open
    // if perms can't be resolved — the backend still gates every API.
    if (!isPublicRoute && globalStore.isLogin) {
        const perms = await loadRbacPerms();
        if (perms && !perms.isSuper && !perms.menus.includes('*')) {
            // Home/dashboard aggregates feature APIs they can't call, so
            // it's effectively off-limits too — send them somewhere usable.
            const wantsHome = to.path === '/' || to.name === 'home';
            if (wantsHome || !isPathAllowed(perms, to.path)) {
                const dest = firstAllowedPath(perms);
                if (dest && dest !== to.path && !to.path.startsWith(dest + '/') && to.path !== dest) {
                    next(dest);
                    NProgress.done();
                    return;
                }
                if (!dest) {
                    next({ name: 'entrance', params: { code: globalStore.entrance } });
                    NProgress.done();
                    return;
                }
            }
        }
    }

    if (to.path === '/apps/all' && to.query.install != undefined) {
        return next();
    }
    const activeMenuKey = 'cachedRoute' + (to.meta.activeMenu || '');
    if (to.query.uncached != undefined) {
        const query = { ...to.query };
        delete query.uncached;
        localStorage.removeItem(activeMenuKey);
        return next({ path: to.path, query });
    }

    const cachedRoute = localStorage.getItem(activeMenuKey);
    if (
        to.meta.activeMenu &&
        to.meta.activeMenu != from.meta.activeMenu &&
        cachedRoute &&
        cachedRoute !== to.path &&
        !isRedirecting
    ) {
        isRedirecting = true;
        next(cachedRoute);
        NProgress.done();
        return;
    }

    if (!to.matched.some((record) => record.meta.requiresAuth)) return next();

    return next();
});

router.afterEach((to) => {
    if (to.meta.activeMenu && !to.meta.ignoreTab && !isRedirecting) {
        let notMathParam = true;
        if (to.matched.some((record) => record.path.includes(':'))) {
            notMathParam = false;
        }
        if (notMathParam) {
            if (to.meta.activeMenu === '/cronjobs' && to.path === '/cronjobs/cronjob/operate') {
                localStorage.setItem('cachedRoute' + to.meta.activeMenu, '/cronjobs/cronjob');
            } else if (to.meta.activeMenu === '/containers' && to.path === '/containers/container/operate') {
                localStorage.setItem('cachedRoute' + to.meta.activeMenu, '/containers/container');
            } else if (to.meta.activeMenu === '/toolbox' && to.path === '/toolbox/clam/setting') {
                localStorage.setItem('cachedRoute' + to.meta.activeMenu, '/toolbox/clam');
            } else {
                localStorage.setItem('cachedRoute' + to.meta.activeMenu, to.path);
            }
        }
    }

    isRedirecting = false;
    NProgress.done();
});

// After a redeploy the hashed chunk filenames change. A tab that still
// runs the previous bundle will fail to lazy-load a route chunk with
// "Failed to fetch dynamically imported module". Recover by doing a
// single hard reload (which pulls the fresh index.html + chunk graph),
// guarded so a genuinely missing chunk can't cause a reload loop.
const isChunkLoadError = (err: unknown): boolean => {
    const msg = err instanceof Error ? err.message : String(err ?? '');
    return (
        /Failed to fetch dynamically imported module/i.test(msg) ||
        /error loading dynamically imported module/i.test(msg) ||
        /Importing a module script failed/i.test(msg)
    );
};

const reloadOnceForStaleChunks = () => {
    const KEY = 'chunk-reload-at';
    const last = Number(sessionStorage.getItem(KEY) || 0);
    // Only auto-reload if we haven't already done so in the last 10s.
    if (Date.now() - last < 10_000) return;
    sessionStorage.setItem(KEY, String(Date.now()));
    window.location.reload();
};

router.onError((err) => {
    if (isChunkLoadError(err)) {
        NProgress.done();
        reloadOnceForStaleChunks();
    }
});

// Lazy imports that fail outside a navigation (e.g. a deferred
// component) surface as an unhandled rejection — catch those too.
window.addEventListener('vite:preloadError', () => {
    reloadOnceForStaleChunks();
});

export default router;
