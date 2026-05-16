import apiClient from './client';
export async function apiSetup(username, password) {
    const res = await apiClient.post('/api/v1/setup', { username, password });
    return res.data;
}
export async function apiLogin(username, password) {
    const res = await apiClient.post('/api/v1/auth/login', { username, password });
    return res.data;
}
export async function apiLogout() {
    await apiClient.post('/api/v1/auth/logout');
}
export async function apiGetMe() {
    const res = await apiClient.get('/api/v1/auth/me');
    return res.data;
}
export async function apiGetCsrf() {
    const res = await apiClient.get('/api/v1/auth/csrf');
    return res.data;
}
export async function apiChangePassword(oldPassword, newPassword) {
    await apiClient.post('/api/v1/auth/password', { oldPassword, newPassword });
}
