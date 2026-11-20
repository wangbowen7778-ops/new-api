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
import type { TaskLog, TaskRequestMedia } from '../types'

function objectValue(value: unknown): Record<string, unknown> | null {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function positiveNumber(value: unknown): number | null {
  const number =
    typeof value === 'number'
      ? value
      : typeof value === 'string' && value.trim() !== ''
        ? Number(value)
        : Number.NaN
  return Number.isFinite(number) && number > 0 ? number : null
}

function textValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function getTaskUpstreamUsage(
  log: TaskLog
): Record<string, unknown> | null {
  return objectValue(objectValue(log.data)?.usage)
}

export function getTaskUpstreamOutput(
  log: TaskLog
): Record<string, unknown> | null {
  return objectValue(objectValue(log.data)?.output)
}

export function getTaskPrompt(log: TaskLog): string {
  const direct = textValue(log.request?.prompt)
  if (direct) return direct
  const properties = textValue(log.properties?.input)
  if (properties) return properties
  return textValue(getTaskUpstreamOutput(log)?.orig_prompt)
}

export function getTaskModelName(log: TaskLog): string {
  return (
    textValue(log.request?.model) ||
    textValue(objectValue(log.data)?.model) ||
    textValue(log.properties?.origin_model_name) ||
    '-'
  )
}

export function getTaskSource(log: TaskLog): string {
  return (
    textValue(log.request?.source) ||
    textValue(log.admin_info?.request_path) ||
    textValue(log.platform) ||
    '-'
  )
}

export function getTaskReferenceMedia(log: TaskLog): TaskRequestMedia[] {
  return log.request?.media?.filter((media) => media.kind) ?? []
}

export interface TaskUsageSeconds {
  total: number | null
  input: number | null
  output: number | null
}

/**
 * Billable seconds for the task detail breakdown. Settled usage facts are the
 * authoritative source; the upstream poll body is used only when the task
 * predates usage facts or the settlement hook omitted the split.
 */
export function getTaskUsageSeconds(log: TaskLog): TaskUsageSeconds {
  const facts = log.billing?.usage_facts ?? {}
  const usage = getTaskUpstreamUsage(log) ?? {}
  // 旧任务没有落拆分事实；若请求里也没有参考视频，整段用量都是输出视频。
  const hasVideoReference = (log.request?.media ?? []).some(
    (media) => media.kind === 'video'
  )
  const total =
    positiveNumber(facts.seconds) ??
    positiveNumber(usage.duration) ??
    positiveNumber(usage.output_video_duration)
  const input =
    positiveNumber(facts.input_video_seconds) ??
    positiveNumber(usage.input_video_duration) ??
    positiveNumber(usage.input_seconds)
  const output =
    positiveNumber(facts.output_seconds) ??
    positiveNumber(facts.output_video_seconds) ??
    positiveNumber(usage.output_video_duration) ??
    positiveNumber(usage.output_seconds) ??
    (total !== null && input !== null ? total - input : null) ??
    (total !== null && !hasVideoReference ? total : null)
  return {
    total:
      total ??
      (input !== null || output !== null ? (input ?? 0) + (output ?? 0) : null),
    input,
    output,
  }
}

export interface TaskParameterValue {
  key: 'resolution' | 'duration' | 'ratio'
  value: string | number | null
  auto?: boolean
}

/**
 * Request parameters for display. Missing request snapshots on older tasks
 * fall back to values reported by the upstream task body.
 */
export function getTaskParameterValues(log: TaskLog): TaskParameterValue[] {
  const parameters = log.request?.parameters ?? {}
  const usage = getTaskUpstreamUsage(log) ?? {}
  const resolution = textValue(parameters.resolution)
  const ratio =
    textValue(parameters.ratio) || textValue(usage.ratio)
  const durationValue = parameters.duration ?? parameters.seconds
  const auto = typeof durationValue === 'number' && durationValue === -1
  const durationFallback =
    positiveNumber(usage.output_video_duration) ?? positiveNumber(usage.duration)
  return [
    {
      key: 'resolution',
      value:
        resolution ||
        (positiveNumber(usage.SR) !== null ? `${usage.SR}P` : null),
    },
    {
      key: 'duration',
      value: auto ? null : (positiveNumber(durationValue) ?? durationFallback),
      auto,
    },
    { key: 'ratio', value: ratio || null },
  ]
}
