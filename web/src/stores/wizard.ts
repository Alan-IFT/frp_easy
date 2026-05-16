import { defineStore } from 'pinia'
import { apiGetWizardStatus, apiWizardComplete } from '../api/wizard'

interface WizardState {
  wizardHandled: boolean
  shouldShow: boolean
  checked: boolean
}

export const useWizardStore = defineStore('wizard', {
  state: (): WizardState => ({
    wizardHandled: false,
    shouldShow: false,
    checked: false,
  }),

  actions: {
    async checkWizard(): Promise<void> {
      try {
        const status = await apiGetWizardStatus()
        this.wizardHandled = status.handled
        this.shouldShow = status.shouldShow
      } catch {
        // On error, don't show wizard
        this.shouldShow = false
      } finally {
        this.checked = true
      }
    },

    async completeWizard(): Promise<void> {
      await apiWizardComplete()
      this.wizardHandled = true
      this.shouldShow = false
    },
  },
})
