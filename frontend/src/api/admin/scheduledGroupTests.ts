/**
 * Admin Scheduled Group Tests API endpoints.
 */

import { apiClient } from '../client'
import type {
  CreateScheduledGroupTestPlanRequest,
  ScheduledGroupTestPlan,
  UpdateScheduledGroupTestPlanRequest
} from '@/types'

export async function list(): Promise<ScheduledGroupTestPlan[]> {
  const { data } = await apiClient.get<ScheduledGroupTestPlan[]>('/admin/scheduled-group-test-plans')
  return data ?? []
}

export async function create(req: CreateScheduledGroupTestPlanRequest): Promise<ScheduledGroupTestPlan> {
  const { data } = await apiClient.post<ScheduledGroupTestPlan>('/admin/scheduled-group-test-plans', req)
  return data
}

export async function update(
  id: number,
  req: UpdateScheduledGroupTestPlanRequest
): Promise<ScheduledGroupTestPlan> {
  const { data } = await apiClient.put<ScheduledGroupTestPlan>(
    `/admin/scheduled-group-test-plans/${id}`,
    req
  )
  return data
}

export async function deletePlan(id: number): Promise<void> {
  await apiClient.delete(`/admin/scheduled-group-test-plans/${id}`)
}

export const scheduledGroupTestsAPI = {
  list,
  create,
  update,
  delete: deletePlan
}

export default scheduledGroupTestsAPI
