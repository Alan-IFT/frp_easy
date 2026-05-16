import apiClient from './client'
import type { WizardStatus } from '../types'

export async function apiGetWizardStatus(): Promise<WizardStatus> {
  const res = await apiClient.get<WizardStatus>('/api/v1/wizard/status')
  return res.data
}

export async function apiWizardComplete(): Promise<void> {
  await apiClient.post('/api/v1/wizard/complete')
}
