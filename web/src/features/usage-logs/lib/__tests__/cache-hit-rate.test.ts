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
import { describe, expect, test } from 'vitest'

import { getCacheHitRate } from '../format'

describe('cache hit rate', () => {
  test('divides cached tokens by prompt tokens under OpenAI semantics', () => {
    // OpenAI logs report cached tokens as part of prompt_tokens.
    expect(getCacheHitRate(1000, 400, 0, null)).toBe(0.4)
  })

  test('adds cache read/write back into the denominator under Anthropic semantics', () => {
    // Anthropic logs report prompt_tokens excluding cache tokens; cache
    // writes count as misses.
    expect(
      getCacheHitRate(100, 800, 100, { usage_semantic: 'anthropic' })
    ).toBe(0.8)
  })

  test('falls back to the claude marker for legacy logs without usage_semantic', () => {
    expect(getCacheHitRate(100, 800, 100, { claude: true })).toBe(0.8)
  })

  test('explicit openai usage_semantic overrides the claude marker', () => {
    expect(
      getCacheHitRate(1000, 400, 0, { claude: true, usage_semantic: 'openai' })
    ).toBe(0.4)
  })

  test('returns null when no cache read happened', () => {
    expect(getCacheHitRate(1000, 0, 500, null)).toBeNull()
  })

  test('returns null when the denominator is zero', () => {
    // Under Anthropic semantics the denominator always contains the cache
    // read tokens, so only the OpenAI denominator can be zero.
    expect(getCacheHitRate(0, 100, 0, null)).toBeNull()
  })
})
