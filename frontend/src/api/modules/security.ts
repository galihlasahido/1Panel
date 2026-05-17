import http from '@/api';
import { Security } from '../interface/security';

export const getSecurityOverview = () => {
    return http.get<Security.Overview>(`/core/security/overview`);
};
