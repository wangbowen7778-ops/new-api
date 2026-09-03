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
export interface ModelPreviewItem {
  model_name: string
  quota_type?: number
  model_ratio?: number
  completion_ratio?: number
  model_price?: number
  supported_endpoint_types?: string[]
}

const PREVIEW_LIMIT = 10

/**
 * Dedupe by trimmed model name (first occurrence wins) and cap the list.
 * The full pricing entry is preserved so callers can render ratio prices.
 */
export function selectModelPreview(
  models: ModelPreviewItem[]
): ModelPreviewItem[] {
  const uniqueModels = new Map<string, ModelPreviewItem>()
  for (const model of models) {
    const name = model.model_name?.trim()
    if (name && !uniqueModels.has(name)) {
      uniqueModels.set(name, { ...model, model_name: name })
    }
  }
  return [...uniqueModels.values()].slice(0, PREVIEW_LIMIT)
}
