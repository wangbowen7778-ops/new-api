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

import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  buildSettingJSON,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
} from '../channel-form'

function channelWithSetting(setting: Record<string, unknown>) {
  return channelSchema.parse({
    id: 1,
    name: 'Channel',
    type: 1,
    key: '',
    status: 1,
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    models: 'gpt-4o',
    group: 'default',
    base_url: 'https://api.example',
    setting: JSON.stringify(setting),
  })
}

describe('rewrite response model channel setting', () => {
  test('defaults to off', () => {
    expect(CHANNEL_FORM_DEFAULT_VALUES.rewrite_response_model).toBe(false)
  })

  test('populates the switch from a saved setting', () => {
    expect(
      transformChannelToFormDefaults(
        channelWithSetting({ rewrite_response_model: true })
      ).rewrite_response_model
    ).toBe(true)
    expect(
      transformChannelToFormDefaults(channelWithSetting({}))
        .rewrite_response_model
    ).toBe(false)
  })

  test('writes the switch into the setting JSON only when enabled', () => {
    const enabled = JSON.parse(
      buildSettingJSON({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        rewrite_response_model: true,
      })
    )
    expect(enabled.rewrite_response_model).toBe(true)

    const disabled = JSON.parse(buildSettingJSON(CHANNEL_FORM_DEFAULT_VALUES))
    expect(disabled.rewrite_response_model).toBe(false)
  })

  test('round-trips the switch through the create payload', () => {
    const payload = transformFormDataToCreatePayload(
      channelFormSchema.parse({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        name: 'Channel',
        key: 'test-key',
        models: 'gpt-4o',
        rewrite_response_model: true,
      })
    )
    const setting = JSON.parse(payload.channel.setting ?? '{}')
    expect(setting.rewrite_response_model).toBe(true)
  })
})
