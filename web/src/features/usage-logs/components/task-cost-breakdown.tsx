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
import { Wallet01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { formatLogQuota, formatNumber } from '@/lib/format'
import { toIntlLocale } from '@/i18n/languages'

import { getTaskUsageSeconds } from '../lib/task-detail'
import type { TaskLog } from '../types'
import { DetailSection } from './task-detail-fields'

function secondsLabel(
  seconds: number,
  locale: string | undefined,
  t: (key: string) => string
): string {
  return `${formatNumber(seconds, locale)} ${t('seconds')}`
}

function settleStatus(log: TaskLog): 'settled' | 'refunded' | 'pending' {
  if (log.status === 'FAILURE') return 'refunded'
  if (log.status === 'SUCCESS') return 'settled'
  return 'pending'
}

function settleStatusLabel(
  status: 'settled' | 'refunded' | 'pending',
  t: (key: string) => string
): string {
  if (status === 'refunded') return t('Refunded')
  if (status === 'settled') return t('Settled')
  return t('Pre-charged')
}

function settleStatusVariant(
  status: 'settled' | 'refunded' | 'pending'
): 'success' | 'danger' | 'warning' {
  if (status === 'refunded') return 'danger'
  if (status === 'settled') return 'success'
  return 'warning'
}

export function TaskCostBreakdown(props: { log: TaskLog }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const quota = props.log.quota
  const { total, input, output } = getTaskUsageSeconds(props.log)
  const status = settleStatus(props.log)
  const settled = status === 'settled'
  const unitQuota =
    total !== null && total > 0 && quota > 0 ? quota / total : null
  const breakdownUnit = settled ? unitQuota : null
  const hasSplit =
    breakdownUnit !== null && (input !== null || output !== null)

  return (
    <DetailSection
      label={t('Cost')}
      icon={
        <HugeiconsIcon
          icon={Wallet01Icon}
          className='text-muted-foreground size-3.5'
          strokeWidth={2}
          aria-hidden='true'
        />
      }
    >
      <div className='flex items-center justify-between gap-2'>
        <span className='text-muted-foreground text-xs'>{t('Charged Amount')}</span>
        <span className='flex items-center gap-1.5'>
          <StatusBadge
            label={formatLogQuota(quota)}
            variant='neutral'
            size='sm'
            copyable={false}
            className='font-semibold tabular-nums'
          />
          <StatusBadge
            label={settleStatusLabel(status, t)}
            variant={settleStatusVariant(status)}
            size='sm'
            copyable={false}
          />
        </span>
      </div>

      {hasSplit && breakdownUnit !== null ? (
        <div className='space-y-1.5 border-t pt-1.5'>
          {input !== null ? (
            <div className='flex items-baseline justify-between gap-2 text-xs'>
              <span className='text-muted-foreground'>{t('Input Video')}</span>
              <span className='text-right'>
                <span className='text-muted-foreground tabular-nums'>
                  {secondsLabel(input, locale, t)} ×{' '}
                  {formatLogQuota(breakdownUnit)} {t('per second')}
                </span>
                <span className='ml-2 font-semibold tabular-nums'>
                  {formatLogQuota(input * breakdownUnit)}
                </span>
              </span>
            </div>
          ) : null}
          {output !== null ? (
            <div className='flex items-baseline justify-between gap-2 text-xs'>
              <span className='text-muted-foreground'>{t('Output Video')}</span>
              <span className='text-right'>
                <span className='text-muted-foreground tabular-nums'>
                  {secondsLabel(output, locale, t)} ×{' '}
                  {formatLogQuota(breakdownUnit)} {t('per second')}
                </span>
                <span className='ml-2 font-semibold tabular-nums'>
                  {formatLogQuota(output * breakdownUnit)}
                </span>
              </span>
            </div>
          ) : null}
        </div>
      ) : null}

      {settled && props.log.billing?.pre_consumed_quota ? (
        <div className='text-muted-foreground border-t pt-1.5 text-[11px]'>
          {t('Pre-consumed')}: {formatLogQuota(props.log.billing.pre_consumed_quota)}
        </div>
      ) : null}
    </DetailSection>
  )
}
