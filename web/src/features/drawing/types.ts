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
export type DrawingMode = 'generate' | 'edit'

export interface DrawingConfig {
  /** Send the most recent generated image as the reference for the next request. */
  reuseContext: boolean
  ratio: string
  n: number
  /** Channel group used for this drawing request; '' until resolved. */
  group: string
}

export interface DrawingGroupInfo {
  desc: string
  ratio: number | string
}

export interface DrawingImage {
  id: string
  src: string
  revisedPrompt?: string
}

export interface DrawingRecord {
  id: string
  mode: DrawingMode
  createdAt: number
  prompt: string
  model: string
  ratio: string
  size: string
  n: number
  group?: string
  images: DrawingImage[]
}

export interface ImageApiData {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface ImageApiResponse {
  created?: number
  data?: ImageApiData[]
  output_format?: string
  error?: { message?: string; code?: string | number }
  message?: string
}

export interface PricingModelBrief {
  model_name: string
  /** 0 = token-based, 1 = per-request (per-call price in USD) */
  quota_type: number
  model_price?: number
  group_ratio?: Record<string, number>
}

export interface PricingResponse {
  success: boolean
  data?: PricingModelBrief[]
  group_ratio?: Record<string, number>
}
