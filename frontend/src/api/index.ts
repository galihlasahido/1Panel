import axios, { AxiosInstance, AxiosError, AxiosRequestConfig, AxiosResponse, InternalAxiosRequestConfig } from 'axios';
import { ResultData } from '@/api/interface';
import { ResultEnum } from '@/enums/http-enum';
import { checkStatus } from './helper/check-status';
import router from '@/routers';
import { GlobalStore } from '@/store';
import { MsgError } from '@/utils/message';
import { encodeBase64 } from '@/utils/base64';
import i18n from '@/lang';
import { changeToLocal } from '@/utils/node';
import { getCookie } from '@/utils/auth';
const globalStore = GlobalStore();

const config = {
    baseURL: import.meta.env.VITE_API_URL as string,
    timeout: ResultEnum.TIMEOUT as number,
    withCredentials: true,
};

// A dead/expired session or invalid CSRF token: the request can't be
// trusted and retrying won't help — the user must re-authenticate.
const isAuthFailure = (response?: AxiosResponse<any>) => {
    if (!response) return false;
    if (response.status === 401) return true;
    const message = (response.data?.message || '').toLowerCase();
    return (
        message.includes('csrf token invalid') ||
        message.includes('session expired') ||
        message.includes('not logged in')
    );
};

const toLogin = () => {
    globalStore.isLogin = false;
    router.push({ name: 'entrance', params: { code: globalStore.entrance } });
};

class RequestHttp {
    service: AxiosInstance;
    public constructor(config: AxiosRequestConfig) {
        this.service = axios.create(config);
        this.service.interceptors.request.use(
            (config: AxiosRequestConfig) => {
                let language = globalStore.language;
                config.headers = {
                    'Accept-Language': language,
                    ...config.headers,
                };
                if (config.headers.CurrentNode == undefined) {
                    config.headers.CurrentNode = encodeURIComponent(globalStore.currentNode);
                } else {
                    config.headers.CurrentNode = encodeURIComponent(String(config.headers.CurrentNode));
                }
                if (
                    config.url === '/core/auth/login' ||
                    config.url === '/core/auth/mfalogin' ||
                    config.url === '/core/auth/passkey/begin' ||
                    config.url === '/core/auth/passkey/finish'
                ) {
                    let entrance = encodeBase64(globalStore.entrance);
                    config.headers.EntranceCode = entrance;
                }
                const method = (config.method || 'get').toUpperCase();
                const requiresToken = !['GET', 'HEAD', 'OPTIONS', 'TRACE'].includes(method);
                if (requiresToken) {
                    const csrfToken = getCookie('pcsrftoken');
                    if (csrfToken) {
                        config.headers['X-CSRF-Token'] = csrfToken;
                        globalStore.csrfToken = csrfToken;
                    }
                }
                return {
                    ...config,
                } as InternalAxiosRequestConfig<any>;
            },
            (error: AxiosError) => {
                return Promise.reject(error);
            },
        );

        this.service.interceptors.response.use(
            (response: AxiosResponse) => {
                const { data } = response;
                // RBAC authorization denials come back as code 403 with an
                // "...not permitted..." / "superadmin only" message. The user
                // IS authenticated — surface the error but do NOT log them
                // out (that bounced restricted sub-users back to login the
                // moment they hit any disallowed API).
                const isRbacDenied =
                    data.code == ResultEnum.FORBIDDEN &&
                    /not permitted|superadmin only/i.test(String(data.message || ''));
                if (isRbacDenied) {
                    MsgError(data.message);
                    return Promise.reject(data);
                }
                if (data.code == ResultEnum.OVERDUE || data.code == ResultEnum.FORBIDDEN) {
                    globalStore.isLogin = false;
                    router.push({
                        name: 'entrance',
                        params: { code: globalStore.entrance },
                    });
                    return Promise.reject(data);
                }
                if (data.code == ResultEnum.EXPIRED) {
                    router.push({ name: 'Expired' });
                    return;
                }
                if (data.code == ResultEnum.ERRXPACK) {
                    globalStore.isProductPro = false;
                    window.location.reload();
                    return Promise.reject(data);
                }
                if (data.code == ResultEnum.NodeUnBind) {
                    changeToLocal();
                    window.location.reload();
                    return;
                }
                if (data.code == ResultEnum.ERRGLOBALLOADING) {
                    globalStore.$patch({
                        isLoading: true,
                        loadingText: data.message,
                    });
                    return;
                } else {
                    if (globalStore.isLoading) {
                        globalStore.isLoading = false;
                    }
                }
                if (data.code == ResultEnum.ERRAUTH) {
                    return data;
                }
                if (data.code && data.code !== ResultEnum.SUCCESS) {
                    if (data.message.toLowerCase().indexOf('operation not permitted') !== -1) {
                        MsgError(i18n.global.t('license.tamperHelper'));
                        return Promise.reject(data);
                    }
                    MsgError(data.message);
                    return Promise.reject(data);
                }
                return data;
            },
            async (error: AxiosError) => {
                const { response } = error;

                if (error.message.indexOf('timeout') !== -1) MsgError(i18n.global.t('commons.msg.requestTimeout'));
                if (response) {
                    switch (response.status) {
                        case 313:
                            router.push({ name: 'Expired' });
                            return;
                        case 401:
                            // Expired/invalid session returned as a real HTTP
                            // 401 (e.g. RBAC mid-session revalidation, session
                            // middleware). Without this case it fell through to
                            // default and surfaced as an uncaught rejection with
                            // no redirect.
                            toLogin();
                            return Promise.reject(error);
                        case 403:
                            if (isAuthFailure(response)) {
                                // Stale CSRF token / not-logged-in: the session
                                // is no longer trusted — re-authenticate instead
                                // of silently failing the request.
                                toLogin();
                                return Promise.reject(error);
                            }
                            if (response.data && response.data['message']) {
                                MsgError(response.data['message']);
                            } else {
                                MsgError(i18n.global.t('commons.res.forbidden'));
                            }
                            return Promise.reject(error);
                        case 500:
                        case 502:
                        case 524:
                        case 407:
                            checkStatus(
                                response.status,
                                response.data && response.data['message'] ? response.data['message'] : '',
                            );
                            return Promise.reject(error);
                        default:
                            return Promise.reject(error);
                    }
                }
                if (!window.navigator.onLine) router.replace({ path: '/500' });
                return Promise.reject(error);
            },
        );
    }

    get<T>(url: string, params?: object, _object = {}): Promise<ResultData<T>> {
        return this.service.get(url, { params, ..._object });
    }
    post<T>(url: string, params?: object, timeout?: number, headers?: object): Promise<ResultData<T>> {
        let config = {
            baseURL: import.meta.env.VITE_API_URL as string,
            timeout: timeout ? timeout : (ResultEnum.TIMEOUT as number),
            withCredentials: true,
            headers: headers,
        };
        if (headers) {
            config.headers = headers;
        }
        return this.service.post(url, params, config);
    }
    postLocalNode<T>(url: string, params?: object, timeout?: number): Promise<ResultData<T>> {
        return this.service.post(url, params, {
            baseURL: import.meta.env.VITE_API_URL as string,
            timeout: timeout ? timeout : (ResultEnum.TIMEOUT as number),
            withCredentials: true,
            headers: {
                CurrentNode: 'local',
            },
        });
    }
    put<T>(url: string, params?: object, _object = {}): Promise<ResultData<T>> {
        return this.service.put(url, params, _object);
    }
    delete<T>(url: string, params?: any, _object = {}): Promise<ResultData<T>> {
        return this.service.delete(url, { params, ..._object });
    }
    download<BlobPart>(url: string, params?: object, _object = {}): Promise<BlobPart> {
        return this.service.post(url, params, _object);
    }
    upload<T>(url: string, params: object = {}, config?: AxiosRequestConfig): Promise<ResultData<T>> {
        return this.service.post(url, params, config);
    }
}

export default new RequestHttp(config);
