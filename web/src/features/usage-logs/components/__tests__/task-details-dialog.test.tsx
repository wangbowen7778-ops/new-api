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
import { render, screen } from '@testing-library/react'
import { describe, expect, test } from 'vitest'

import type { TaskLog } from '../../types'
import { TaskDetailsDialog } from '../dialogs/task-details-dialog'

function makeTask(properties: TaskLog['properties']): TaskLog {
  return {
    id: 1,
    user_id: 7,
    task_id: 'task_public',
    action: 'GENERATE',
    channel_id: 3,
    group: 'default',
    quota: 100,
    submit_time: 1,
    status: 'SUCCESS',
    platform: 'sora',
    properties,
  }
}

function renderTask(properties: TaskLog['properties']): void {
  render(
    <TaskDetailsDialog
      log={makeTask(properties)}
      isAdmin
      isRoot
      open
      onOpenChange={() => undefined}
    />
  )
}

describe('task details model disclosure', () => {
  test('shows the real upstream model when the channel does not rewrite it', () => {
    renderTask({
      origin_model_name: 'requested-model',
      upstream_model_name: 'real-upstream-model',
    })

    expect(screen.getByText('Actual Model')).toBeInTheDocument()
    expect(screen.getByText('real-upstream-model')).toBeInTheDocument()
  })

  test('hides the real upstream model when the channel rewrites the response model', () => {
    renderTask({
      origin_model_name: 'requested-model',
      upstream_model_name: 'real-upstream-model',
      rewrite_response_model: true,
    })

    expect(screen.queryByText('Actual Model')).toBeNull()
    expect(screen.queryByText('real-upstream-model')).toBeNull()
    expect(screen.getByText('requested-model')).toBeInTheDocument()
  })
})
