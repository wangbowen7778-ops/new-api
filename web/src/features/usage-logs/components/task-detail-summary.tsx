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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { formatNumber, formatTimestampToDate } from '@/lib/format'
import { toIntlLocale } from '@/i18n/languages'

import { taskStatusMapper } from '../lib/mappers'
import {
  getTaskModelName,
  getTaskParameterValues,
  getTaskPrompt,
  getTaskReferenceMedia,
  getTaskSource,
} from '../lib/task-detail'
import type { TaskLog, TaskRequestMedia } from '../types'
import { DetailRow, DetailSection } from './task-detail-fields'

function ReferenceMediaItem(props: { media: TaskRequestMedia }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const duration =
    props.media.duration_seconds && props.media.duration_seconds > 0
      ? props.media.duration_seconds
      : null

  if (!props.media.url) {
    return (
      <div className='bg-muted/40 text-muted-foreground flex h-24 min-w-0 flex-col items-center justify-center gap-1 rounded-md border px-2 text-[11px]'>
        {props.media.asset_id ? (
          <>
            <span className='text-foreground text-xs'>
              {t('Material asset')}
            </span>
            <span className='w-full truncate text-center font-mono text-[10px]'>
              {props.media.asset_id}
            </span>
          </>
        ) : props.media.filename ? (
          <>
            <span className='text-foreground text-xs'>
              {t('Uploaded file')}
            </span>
            <span className='w-full truncate text-center font-mono text-[10px]'>
              {props.media.filename}
            </span>
          </>
        ) : (
          <span className='text-center'>
            {props.media.inline
              ? t('Inline content not stored')
              : t('Preview unavailable')}
          </span>
        )}
      </div>
    )
  }

  return (
    <div className='bg-muted/40 relative min-w-0 overflow-hidden rounded-md border'>
      {props.media.kind === 'video' ? (
        <video
          src={props.media.url}
          controls
          preload='metadata'
          className='h-24 w-full bg-black object-cover'
          aria-label={t('Video')}
        />
      ) : (
        <a
          href={props.media.url}
          target='_blank'
          rel='noopener noreferrer'
          className='block'
        >
          <img
            src={props.media.url}
            alt={t('Image')}
            loading='lazy'
            className='h-24 w-full object-cover'
          />
        </a>
      )}
      {duration !== null ? (
        <span className='bg-background/90 absolute right-1 bottom-1 rounded px-1 py-0.5 text-[10px] font-medium tabular-nums'>
          {formatNumber(duration, locale)} {t('seconds')}
        </span>
      ) : null}
    </div>
  )
}

function ReferenceMediaGrid(props: { media: TaskRequestMedia[] }) {
  return (
    <div className='grid min-w-0 grid-cols-2 gap-2'>
      {props.media.map((media, index) => (
        <ReferenceMediaItem
          key={`${media.kind}-${media.url ?? 'inline'}-${index}`}
          media={media}
        />
      ))}
    </div>
  )
}

export function TaskDetailSummary(props: { log: TaskLog }) {
  const { t } = useTranslation()
  const media = getTaskReferenceMedia(props.log)
  const prompt = getTaskPrompt(props.log)
  const parameters = getTaskParameterValues(props.log)

  return (
    <>
      <DetailSection label={t('Basic Information')}>
        <DetailRow label={t('Task ID')} value={props.log.task_id} mono />
        <DetailRow label={t('Source API')} value={getTaskSource(props.log)} mono />
        <DetailRow
          label={t('Submit Time')}
          value={
            props.log.submit_time
              ? formatTimestampToDate(props.log.submit_time, 'seconds')
              : '-'
          }
          mono
        />
        <DetailRow label={t('Model')} value={getTaskModelName(props.log)} mono />
        <DetailRow
          label={t('Status')}
          value={
            <StatusBadge
              label={t(
                taskStatusMapper.getLabel(
                  props.log.status,
                  props.log.status || 'Submitting'
                )
              )}
              variant={taskStatusMapper.getVariant(props.log.status)}
              size='sm'
              copyable={false}
            />
          }
        />
      </DetailSection>

      {prompt ? (
        <DetailSection label={t('Prompt')}>
          <p className='text-xs whitespace-pre-wrap break-words'>{prompt}</p>
        </DetailSection>
      ) : null}

      {media.length > 0 ? (
        <DetailSection label={`${t('Reference Materials')} (${media.length})`}>
          <ReferenceMediaGrid media={media} />
        </DetailSection>
      ) : null}

      {parameters.length > 0 ? (
        <DetailSection label={t('Parameters')}>
          {parameters.map((parameter) => (
            <DetailRow
              key={parameter.key}
              label={t(parameterLabelKey(parameter.key))}
              value={parameterDisplay(parameter.value, parameter.auto, t)}
              mono
            />
          ))}
        </DetailSection>
      ) : null}
    </>
  )
}

function parameterLabelKey(
  key: 'resolution' | 'duration' | 'ratio'
): string {
  switch (key) {
    case 'resolution':
      return 'Resolution'
    case 'duration':
      return 'Duration (seconds)'
    case 'ratio':
      return 'Aspect Ratio'
  }
}

function parameterDisplay(
  value: string | number | null,
  auto: boolean | undefined,
  t: (key: string) => string
): string {
  if (auto) return t('Auto')
  if (value === null) return '-'
  return String(value)
}
