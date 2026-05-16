import { defineStore } from 'pinia';
import { apiGetMe, apiGetCsrf, apiLogin, apiLogout, apiSetup } from '../api/auth';
import { extractErrorMessage } from '../api/client';
export const useAuthStore = defineStore('auth', {
    state: () => ({
        user: null,
        csrfToken: '',
    }),
    actions: {
        async fetchCsrf() {
            try {
                const res = await apiGetCsrf();
                this.csrfToken = res.csrfToken;
            }
            catch {
                // 忽略——无 session 时保持空值
            }
        },
        async checkMe() {
            try {
                const res = await apiGetMe();
                this.user = res.username;
                await this.fetchCsrf();
                return true;
            }
            catch {
                this.user = null;
                return false;
            }
        },
        async setup(username, password) {
            await apiSetup(username, password);
            // setup 成功后自动已登录 → 获取 CSRF
            this.user = username;
            await this.fetchCsrf();
        },
        async login(username, password) {
            await apiLogin(username, password);
            this.user = username;
            await this.fetchCsrf();
        },
        async logout() {
            try {
                await apiLogout();
            }
            catch (e) {
                // 尽力执行
                console.warn('Logout error:', extractErrorMessage(e));
            }
            finally {
                this.user = null;
                this.csrfToken = '';
            }
        },
    },
});
