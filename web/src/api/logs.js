import apiClient from './client';
export async function apiGetLogsTail(kind, lines = 500) {
    const res = await apiClient.get(`/api/v1/logs/${kind}`, {
        params: { lines },
    });
    return res.data;
}
export async function apiGetLogsIncremental(kind, offset) {
    const res = await apiClient.get(`/api/v1/logs/${kind}`, {
        params: { offset },
    });
    return res.data;
}
