/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

import { API_ENDPOINTS } from './constants'
import type {
  DrawingGroupInfo,
  ImageApiResponse,
  PricingResponse,
} from './types'

export interface ImageGenerationPayload {
  model: string
  prompt: string
  n?: number
  size?: string
  quality?: string
  group?: string
}

export async function generateImages(
  payload: ImageGenerationPayload,
  signal?: AbortSignal
): Promise<ImageApiResponse> {
  const res = await api.post(API_ENDPOINTS.IMAGES_GENERATIONS, payload, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function editImages(
  form: FormData,
  signal?: AbortSignal
): Promise<ImageApiResponse> {
  const res = await api.post(API_ENDPOINTS.IMAGES_EDITS, form, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Balance and the user's own group. The user's group is the fallback for the
 * drawing group selector; the selected group is sent with each request.
 */
export async function getUserAccountInfo(): Promise<{
  quota: number
  group: string
} | null> {
  const res = await api.get(API_ENDPOINTS.USER_SELF, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  const { data } = res

  if (
    !data?.success ||
    typeof data.data?.quota !== 'number' ||
    typeof data.data?.group !== 'string'
  ) {
    return null
  }
  return { quota: data.data.quota, group: data.data.group }
}

/**
 * Groups the current user can use, with their ratio and description. The
 * drawing page sends the selected group with each request so channel
 * selection and billing follow the chosen group.
 */
export async function getUserGroups(): Promise<{
  success: boolean
  data?: Record<string, DrawingGroupInfo>
}> {
  const res = await api.get(API_ENDPOINTS.USER_GROUPS, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

export async function getPricing(): Promise<PricingResponse> {
  const res = await api.get(API_ENDPOINTS.PRICING, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data as PricingResponse
}

export function extractErrorMessage(error: unknown): string {
  const requestError = error as {
    response?: { data?: { error?: { message?: string }; message?: string } }
    message?: string
  }

  return (
    requestError?.response?.data?.error?.message ||
    requestError?.response?.data?.message ||
    requestError?.message ||
    'Request error occurred'
  )
}
