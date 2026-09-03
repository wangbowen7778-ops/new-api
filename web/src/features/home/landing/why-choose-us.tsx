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

const DARK_STATS = [
  {
    value: 'As low as 10% of official prices',
    copy: 'Per-model pricing is configured in the console and always matches the pricing page.',
    accent: '#a78bfa',
  },
  {
    value: '99.9% availability',
    copy: 'Multi-channel failover keeps every request on the fastest healthy route.',
    accent: '#34d399',
  },
  {
    value: 'Millisecond channel switching',
    copy: 'Lightweight relay with intelligent routing across channels and providers.',
    accent: '#7dd3fc',
  },
] as const

const RIGHT_POINTS = [
  {
    title: 'Cost-effective',
    copy: 'Transparent per-model pricing with real-time usage logs, saving up to 90% versus official prices.',
  },
  {
    title: 'Pay as you go',
    copy: 'Top up whenever you need — no subscription lock-in, and credits stay valid.',
  },
  {
    title: 'Multi-protocol compatible',
    copy: 'OpenAI, Claude and Gemini compatible routes work out of the box, no migration needed.',
  },
] as const

/**
 * "Why choose us" panel: a dark stat column paired with light reason rows,
 * emphasizing savings and stability for a relay service.
 */
export function WhyChooseUs() {
  const { t } = useTranslation()

  return (
    <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView>
          <h2 className='landing-section-title text-center'>
            {t('Why choose us')}
          </h2>
        </AnimateInView>

        <div className='mt-12 grid gap-5 lg:grid-cols-2'>
          {/* Dark stat panel */}
          <AnimateInView>
            <div className='relative flex h-full flex-col justify-between gap-8 overflow-hidden rounded-[1.75rem] bg-[#26224a] p-8 text-white shadow-[0_24px_70px_rgba(38,34,74,0.35)] sm:p-10'>
              {/* Decorative corner mosaic */}
              <div
                aria-hidden
                className='absolute top-0 right-0 grid grid-cols-3 gap-1.5 p-6'
              >
                {[1, 0, 1, 0, 1, 1, 1, 0, 0].map((cell, index) => (
                  <span
                    className={`size-5 rounded-md ${cell === 1 ? 'bg-white/10' : 'bg-white/4'}`}
                    key={index}
                  />
                ))}
              </div>
              {DARK_STATS.map((stat) => (
                <div key={stat.value}>
                  <p
                    className='text-3xl font-semibold tracking-tight sm:text-4xl'
                    style={{ color: stat.accent }}
                  >
                    {t(stat.value)}
                  </p>
                  <p className='mt-2 max-w-sm text-sm leading-6 text-white/65'>
                    {t(stat.copy)}
                  </p>
                </div>
              ))}
            </div>
          </AnimateInView>

          {/* Light reason rows */}
          <div className='flex flex-col gap-5'>
            {RIGHT_POINTS.map((point, index) => (
              <AnimateInView delay={index * 90} key={point.title}>
                <div className='h-full rounded-[1.75rem] border border-[#101828]/8 bg-white/80 p-8 shadow-[0_16px_48px_rgba(16,24,40,0.05)] sm:p-9'>
                  <h3 className='text-xl font-semibold tracking-tight text-[#101828]'>
                    {t(point.title)}
                  </h3>
                  <p className='mt-3 max-w-md text-sm leading-7 text-[#667085] sm:text-base'>
                    {t(point.copy)}
                  </p>
                </div>
              </AnimateInView>
            ))}
          </div>
        </div>
      </div>
    </section>
  )
}
