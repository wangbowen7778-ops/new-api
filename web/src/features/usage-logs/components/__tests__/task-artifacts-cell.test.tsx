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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import type { TaskLog } from '../../types'
import { TaskArtifactsCell } from '../task-artifacts'

vi.mock('../../api', () => ({
  getTaskArtifacts: vi.fn().mockResolvedValue({
    artifacts: [
      {
        key: 'video',
        type: 'video',
        content_url: 'https://cdn.example/result.mp4',
      },
    ],
    legacyContentUrl: undefined,
    legacyAudioClips: [],
  }),
}))

function taskLog(overrides: Partial<TaskLog> = {}): TaskLog {
  return {
    id: 1,
    user_id: 7,
    platform: 'alibaba',
    task_id: 'task-public',
    action: 'text_to_image',
    channel_id: 357,
    group: 'default',
    quota: 25000,
    submit_time: 1,
    status: 'SUCCESS',
    admin_info: {
      task_plugin: {
        key: 'alibaba',
        name: 'Alibaba Bailian',
        version: '1.4.0',
      },
    },
    ...overrides,
  }
}

function renderCell(log: TaskLog) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <TaskArtifactsCell log={log} />
    </QueryClientProvider>
  )
}

describe('task artifacts cell', () => {
  test('given a synchronous result the gateway did not retain, a hint replaces the artifacts button', () => {
    renderCell(taskLog({ result_discarded: true }))

    expect(screen.getByText('Result not retained')).toBeVisible()
    expect(
      screen.queryByRole('button', { name: /Artifacts/ })
    ).not.toBeInTheDocument()
  })

  test('given a retained successful plugin task, the artifacts button is offered', () => {
    renderCell(taskLog())

    expect(screen.getByRole('button', { name: /Artifacts/ })).toBeVisible()
    expect(screen.queryByText('Result not retained')).not.toBeInTheDocument()
  })

  test('given an unfinished task, neither the button nor the hint is shown', () => {
    renderCell(taskLog({ status: 'IN_PROGRESS', result_discarded: true }))

    expect(screen.getByText('-')).toBeVisible()
    expect(screen.queryByText('Result not retained')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /Artifacts/ })
    ).not.toBeInTheDocument()
  })

  test('given a settled video task, the artifacts dialog shows request media and the cost split', async () => {
    renderCell(
      taskLog({
        quota: 780000,
        request: {
          source: '/v1/videos',
          model: 'wan3.0-video',
          prompt: 'flowers in the breeze',
          parameters: { resolution: '480P', ratio: '9:16' },
          media: [
            {
              type: 'reference_video',
              kind: 'video',
              url: 'https://cdn.example/ref.mp4',
              duration_seconds: 5,
            },
            {
              type: 'reference_image',
              kind: 'image',
              url: 'https://cdn.example/frame.png',
            },
          ],
        },
        billing: {
          pre_consumed_quota: 2025000,
          usage_facts: {
            seconds: 10,
            input_video_seconds: 5,
            output_seconds: 5,
          },
        },
      })
    )

    fireEvent.click(screen.getByRole('button', { name: /Artifacts/ }))

    expect(
      await screen.findByText(/Reference Materials \(2\)/)
    ).toBeVisible()
    expect(screen.getByText('flowers in the breeze')).toBeVisible()
    expect(screen.getByText('Input Video')).toBeVisible()
    expect(screen.getByText('Output Video')).toBeVisible()
    expect(screen.getByText('Charged Amount')).toBeVisible()
    expect(screen.getByText('Settled')).toBeVisible()
    expect(screen.getAllByText(/5 seconds/).length).toBeGreaterThan(0)
  })

  test('given a task without a request snapshot, the prompt falls back to the upstream body', async () => {
    renderCell(
      taskLog({
        data: {
          output: { orig_prompt: 'a red cube rotates slowly' },
          usage: { duration: 2, output_video_duration: 2, SR: 480 },
        },
      })
    )

    fireEvent.click(screen.getByRole('button', { name: /Artifacts/ }))

    expect(
      await screen.findByText('a red cube rotates slowly')
    ).toBeVisible()
    expect(screen.queryByText(/Reference Materials/)).not.toBeInTheDocument()
  })
})
