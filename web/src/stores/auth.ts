import { defineStore } from 'pinia'
import { apiGetMe, apiGetCsrf, apiLogin, apiLogout, apiSetup } from '../api/auth'
import { extractErrorMessage } from '../api/client'

interface AuthState {
  user: string | null
  csrfToken: string
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    user: null,
    csrfToken: '',
  }),

  actions: {
    async fetchCsrf(): Promise<void> {
      try {
        const res = await apiGetCsrf()
        this.csrfToken = res.csrfToken
      } catch {
        // ignore — セッションがない場合は空のまま
      }
    },

    async checkMe(): Promise<boolean> {
      try {
        const res = await apiGetMe()
        this.user = res.username
        await this.fetchCsrf()
        return true
      } catch {
        this.user = null
        return false
      }
    },

    async setup(username: string, password: string): Promise<void> {
      await apiSetup(username, password)
      // setup 成功後は自動ログイン済み → CSRF 取得
      this.user = username
      await this.fetchCsrf()
    },

    async login(username: string, password: string): Promise<void> {
      await apiLogin(username, password)
      this.user = username
      await this.fetchCsrf()
    },

    async logout(): Promise<void> {
      try {
        await apiLogout()
      } catch (e) {
        // ベストエフォート
        console.warn('Logout error:', extractErrorMessage(e))
      } finally {
        this.user = null
        this.csrfToken = ''
      }
    },
  },
})
