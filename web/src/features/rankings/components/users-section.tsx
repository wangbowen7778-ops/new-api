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
import { VChart } from '@visactor/react-vchart'
import { Users } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'

import { formatTokens } from '../lib/format'
import type { RankingPeriod, UserRanking } from '../types'

const PERIOD_DESCRIPTIONS: Record<RankingPeriod, string> = {
  today: 'Hourly token usage by user across the last 24 hours',
  week: 'Weekly token usage by user across the past few weeks',
  month: 'Daily token usage by user across the past month',
  year: 'Weekly token usage by user across the past year',
}

// Rank 1..5 bar colours, strongest -> lightest, readable on both themes.
const TOP_USER_BAR_COLOURS = [
  '#6366f1',
  '#818cf8',
  '#a5b4fc',
  '#c7d2fe',
  '#e0e7ff',
]

type UsersSectionProps = {
  rows: UserRanking[]
  period: RankingPeriod
}

/**
 * "Top Users" card: a horizontal bar chart of the top 5 users by token usage,
 * ranked 1..5 top to bottom. Names are already masked server-side.
 */
export function UsersSection(props: UsersSectionProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const chartTextColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.68)'
      : 'rgba(15, 23, 42, 0.58)'
  const chartGridColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.12)'
      : 'rgba(15, 23, 42, 0.12)'

  const colourMap = useMemo(() => {
    const map: Record<string, string> = {}
    props.rows.forEach((row, idx) => {
      map[row.display_name] =
        TOP_USER_BAR_COLOURS[idx % TOP_USER_BAR_COLOURS.length]
    })
    return map
  }, [props.rows])

  const spec = useMemo(() => {
    if (props.rows.length === 0) return null
    return {
      type: 'bar' as const,
      data: [
        {
          id: 'top-users',
          // Data is already ordered 1..5; VChart lays the first item at the
          // top, so rank 1 renders as the largest bar at the top.
          values: props.rows.map((row) => ({
            name: row.display_name,
            tokens: row.total_tokens,
          })),
        },
      ],
      xField: 'tokens',
      yField: 'name',
      direction: 'horizontal' as const,
      seriesField: 'name',
      legends: { visible: false },
      color: { specified: colourMap },
      paddingInner: 0.35,
      label: {
        visible: true,
        position: 'right',
        style: { fill: chartTextColor, fontSize: 11, fontWeight: 600 },
        formatMethod: (val: number | string) => formatTokens(Number(val) || 0),
      },
      axes: [
        {
          orient: 'bottom',
          label: {
            formatMethod: (val: number | string) =>
              formatTokens(Number(val) || 0),
            style: { fill: chartTextColor, fontSize: 10 },
          },
          tick: { visible: false },
          grid: {
            visible: true,
            style: { lineDash: [3, 3], stroke: chartGridColor },
          },
        },
        {
          orient: 'left',
          label: {
            style: { fill: chartTextColor, fontSize: 11 },
            autoLimit: true,
          },
          tick: { visible: false },
          domainLine: { visible: false },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: (datum: Record<string, unknown>) =>
                String(datum?.name ?? ''),
              value: (datum: Record<string, unknown>) =>
                formatTokens(Number(datum?.tokens) || 0),
            },
          ],
        },
      },
    }
  }, [props.rows, colourMap, chartTextColor, chartGridColor])

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <header className='px-5 py-4'>
        <h2 className='text-foreground inline-flex items-center gap-2 text-base font-semibold'>
          <Users className='text-primary size-4' />
          {t('Top Users')}
        </h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t(PERIOD_DESCRIPTIONS[props.period])}
        </p>
      </header>

      {props.rows.length === 0 ? (
        <div className='text-muted-foreground/80 px-5 pb-8 text-center text-sm'>
          {t('No user data available')}
        </div>
      ) : (
        <div className='px-5 pb-5'>
          <div className='h-52'>
            {themeReady && spec ? (
              <VChart
                key={`users-chart-${resolvedTheme}-${props.period}`}
                spec={{
                  ...spec,
                  theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                  background: 'transparent',
                }}
                option={VCHART_OPTION}
              />
            ) : (
              <div className='text-muted-foreground/80 flex h-full items-center justify-center text-xs'>
                {t('No user data available')}
              </div>
            )}
          </div>
        </div>
      )}
    </section>
  )
}
