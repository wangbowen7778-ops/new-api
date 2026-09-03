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

import { AnimateInView } from '@/components/animate-in-view'

import { CountUp } from './count-up'

/** Illustrative weekly latency profile in milliseconds. */
const LATENCY_BARS = [212, 198, 240, 176, 204, 168, 184] as const

/** SVG ring geometry for the availability donut. */
const RING_RADIUS = 54
const RING_CIRCUMFERENCE = 2 * Math.PI * RING_RADIUS
const AVAILABILITY = 99.9

/**
 * Stability showcase: availability ring, weekly response-time bars and
 * success rate. The metrics are illustrative placeholders — the panel is
 * explicitly labelled as demo data.
 */
export function StabilityMetrics() {
  const { t } = useTranslation()

  return (
    <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView>
          <div className='mx-auto max-w-3xl text-center'>
            <p className='landing-section-kicker'>{t('Reliability')}</p>
            <h2 className='landing-section-title'>
              {t('Stability you can put in front of customers.')}
            </h2>
            <p className='landing-section-copy'>
              {t(
                'Multi-channel failover and intelligent routing keep every request on the fastest healthy route, around the clock.'
              )}
            </p>
          </div>
        </AnimateInView>

        <div className='mt-12 grid gap-4 md:grid-cols-3'>
          <AnimateInView delay={0}>
            <div className='landing-metric-card flex h-full flex-col items-center justify-center text-center'>
              <div className='relative'>
                <svg
                  aria-hidden
                  className='landing-metric-ring'
                  height='132'
                  viewBox='0 0 132 132'
                  width='132'
                >
                  <circle
                    cx='66'
                    cy='66'
                    fill='none'
                    r={RING_RADIUS}
                    stroke='#edf2f7'
                    strokeWidth='9'
                  />
                  <circle
                    className='landing-metric-ring-value'
                    cx='66'
                    cy='66'
                    fill='none'
                    r={RING_RADIUS}
                    stroke='url(#landing-ring-gradient)'
                    strokeLinecap='round'
                    strokeWidth='9'
                    style={
                      {
                        '--ring-circumference': RING_CIRCUMFERENCE,
                        '--ring-progress': AVAILABILITY / 100,
                      } as React.CSSProperties
                    }
                    transform='rotate(-90 66 66)'
                  />
                  <defs>
                    <linearGradient
                      id='landing-ring-gradient'
                      x1='0'
                      x2='1'
                      y1='0'
                      y2='1'
                    >
                      <stop offset='0' stopColor='#7c3aed' />
                      <stop offset='1' stopColor='#12b76a' />
                    </linearGradient>
                  </defs>
                </svg>
                <div className='absolute inset-0 flex items-center justify-center'>
                  <span className='landing-font-instrument text-3xl tracking-tight text-[#101828]'>
                    <CountUp decimals={1} suffix='%' value={AVAILABILITY} />
                  </span>
                </div>
              </div>
              <p className='mt-4 text-sm font-semibold text-[#101828]'>
                {t('Availability')}
              </p>
              <p className='mt-1 text-xs text-[#98a2b3]'>
                {t('Across all channels')}
              </p>
            </div>
          </AnimateInView>

          <AnimateInView delay={100}>
            <div className='landing-metric-card flex h-full flex-col justify-between'>
              <div>
                <p className='text-sm font-semibold text-[#101828]'>
                  {t('Average response')}
                </p>
                <p className='mt-1 text-xs text-[#98a2b3]'>{t('Last 7 days')}</p>
              </div>
              <div className='mt-6 flex h-24 items-end gap-2'>
                {LATENCY_BARS.map((value, index) => (
                  <div
                    className='landing-metric-bar flex-1 rounded-t-md bg-gradient-to-t from-[#7c3aed]/35 to-[#7c3aed]'
                    key={index}
                    style={{
                      height: `${(value / 240) * 100}%`,
                      animationDelay: `${index * 70}ms`,
                    }}
                    title={`${value}ms`}
                  />
                ))}
              </div>
              <p className='landing-font-instrument mt-5 text-3xl tracking-tight text-[#101828]'>
                <CountUp suffix='ms' value={184} />
              </p>
            </div>
          </AnimateInView>

          <AnimateInView delay={200}>
            <div className='landing-metric-card flex h-full flex-col items-center justify-center text-center'>
              <span className='landing-font-instrument text-5xl tracking-tight text-[#101828]'>
                <CountUp decimals={2} suffix='%' value={99.98} />
              </span>
              <p className='mt-3 text-sm font-semibold text-[#101828]'>
                {t('Success rate')}
              </p>
              <p className='mt-1 text-xs text-[#98a2b3]'>
                {t('Requests completed without error')}
              </p>
            </div>
          </AnimateInView>
        </div>

        <AnimateInView animation='fade-in'>
          <p className='mt-8 text-center text-xs text-[#98a2b3]'>
            {t('Illustrative metrics — replace with your own production stats.')}
          </p>
        </AnimateInView>
      </div>
    </section>
  )
}
