import { defineStore } from 'pinia';
import { apiGetWizardStatus, apiWizardComplete } from '../api/wizard';
export const useWizardStore = defineStore('wizard', {
    state: () => ({
        wizardHandled: false,
        shouldShow: false,
        checked: false,
    }),
    actions: {
        async checkWizard() {
            try {
                const status = await apiGetWizardStatus();
                this.wizardHandled = status.handled;
                this.shouldShow = status.shouldShow;
            }
            catch {
                // On error, don't show wizard
                this.shouldShow = false;
            }
            finally {
                this.checked = true;
            }
        },
        async completeWizard() {
            await apiWizardComplete();
            this.wizardHandled = true;
            this.shouldShow = false;
        },
    },
});
