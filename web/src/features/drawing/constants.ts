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
import type { DrawingConfig } from './types'

export const API_ENDPOINTS = {
  IMAGES_GENERATIONS: '/pg/images/generations',
  IMAGES_EDITS: '/pg/images/edits',
  USER_SELF: '/api/user/self',
  USER_GROUPS: '/api/user/self/groups',
  PRICING: '/api/pricing',
} as const

/**
 * Single image model; billing is the model's own per-call/token price and is
 * not differentiated by resolution. Quality is fixed upstream.
 */
export const DRAWING_MODEL = 'gpt-image-2'

export const FIXED_QUALITY = 'medium' as const

// Long-side pixel budget; the aspect ratio distributes it.
export const DRAWING_LONG_SIDE = 1024

export const ASPECT_RATIOS = [
  '1:1',
  '3:2',
  '2:3',
  '4:3',
  '3:4',
  '16:9',
  '9:16',
  '3:1',
  '1:3',
] as const

export const DEFAULT_ASPECT_RATIO = '1:1' as const

export function computeSize(ratio: string): string {
  const [ratioW, ratioH] = ratio.split(':').map(Number)
  if (!ratioW || !ratioH) {
    return `${DRAWING_LONG_SIDE}x${DRAWING_LONG_SIDE}`
  }
  const short = Math.max(
    256,
    Math.round(
      (DRAWING_LONG_SIDE * Math.min(ratioW, ratioH)) / Math.max(ratioW, ratioH),
    ),
  )
  return ratioW >= ratioH
    ? `${DRAWING_LONG_SIDE}x${short}`
    : `${short}x${DRAWING_LONG_SIDE}`
}

export const N_OPTIONS = [1, 2, 3, 4] as const

export const DEFAULT_CONFIG: DrawingConfig = {
  reuseContext: true,
  ratio: DEFAULT_ASPECT_RATIO,
  n: 1,
  group: '',
}

export const STORAGE_KEYS = {
  CONFIG: 'drawing_config',
  HISTORY: 'drawing_history',
} as const

// IndexedDB quota is far larger than localStorage, so history is bounded only
// by record count; trim the oldest records first.
export const MAX_HISTORY_ITEMS = 100

export const ERROR_MESSAGES = {
  API_REQUEST_ERROR: 'Request error occurred',
  PROMPT_REQUIRED: 'Please enter a prompt',
  IMAGE_REQUIRED: 'Please upload an image to edit',
} as const
