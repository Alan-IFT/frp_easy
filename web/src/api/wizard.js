import apiClient from './client';
export async function apiGetWizardStatus() {
    const res = await apiClient.get('/api/v1/wizard/status');
    return res.data;
}
export async function apiWizardComplete() {
    await apiClient.post('/api/v1/wizard/complete');
}
