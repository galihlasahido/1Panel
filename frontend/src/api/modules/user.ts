import http from '@/api';
import { ResPage } from '../interface';
import { UserMgmt } from '../interface/user';

export const searchUsers = (params: UserMgmt.UserSearch) => {
    return http.post<ResPage<UserMgmt.UserInfo>>(`/core/users/search`, params);
};

export const createUser = (params: UserMgmt.UserCreate) => {
    return http.post<UserMgmt.UserInfo>(`/core/users/create`, params);
};

export const updateUser = (params: UserMgmt.UserUpdate) => {
    return http.post(`/core/users/update`, params);
};

export const updateUserPassword = (id: number, password: string) => {
    return http.post(`/core/users/update/password`, { id, password });
};

export const deleteUser = (id: number) => {
    return http.post(`/core/users/del/${id}`, {});
};

export const getCurrentUser = () => {
    return http.get<{ name: string; isSuper: boolean; menus: string[]; nodes: string[] }>(`/core/auth/me`);
};
