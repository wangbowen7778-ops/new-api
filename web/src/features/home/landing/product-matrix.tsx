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
import { ArrowRight01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { cn } from '@/lib/utils'

interface MatrixCard {
  title: string
  copy: string
  ctaLabel: string
  ctaTo: '/pricing' | '/docs' | null
  dark: boolean
}

const CARDS: MatrixCard[] = [
  {
    title: 'Ready-to-use model API',
    copy: 'Chat, reasoning, drawing and audio over one unified API — pay as you go, launch in minutes.',
    ctaLabel: 'Get Started',
    ctaTo: null,
    dark: true,
  },
  {
    title: 'Multi-channel smart routing',
    copy: 'Requests automatically follow the fastest healthy channel, with seamless failover on errors.',
    ctaLabel: 'View documentation',
    ctaTo: '/docs',
    dark: false,
  },
  {
    title: 'Unified billing, full visibility',
    copy: 'Every call is itemized with per-model pricing, so usage and cost stay fully auditable.',
    ctaLabel: 'View Pricing',
    ctaTo: '/pricing',
    dark: false,
  },
  {
    title: 'Scoped keys and permissions',
    copy: 'Issue tokens with group, quota and expiry controls, keeping team access clean and safe.',
    ctaLabel: 'Get Started',
    ctaTo: null,
    dark: true,
  },
]

/** Decorative pixel mosaic in the card corner. */
function PixelArt(props: { dark: boolean }) {
  const pattern = [0, 1, 0, 1, 1, 0, 0, 1, 1, 1, 0, 1]
  return (
    <div
      aria-hidden
      className='pointer-events-none absolute right-6 bottom-6 grid grid-cols-3 gap-1.5'
    >
      {pattern.map((cell, index) => (
        <span
          className={cn(
            'size-6 rounded-md transition-transform duration-300',
            cell === 1
              ? props.dark
                ? 'bg-white/12'
                : 'bg-[#7c3aed]/12'
              : props.dark
                ? 'bg-white/4'
                : 'bg-[#7c3aed]/4'
          )}
          key={index}
        />
      ))}
    </div>
  )
}

export function ProductMatrix(props: { ctaTo: '/dashboard' | '/sign-up' }) {
  const { t } = useTranslation()

  return (
    <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
      <div className='mx-auto max-w-7xl'>
        <AnimateInView>
          <div className='mx-auto max-w-3xl text-center'>
            <p className='landing-section-kicker'>{t('Product matrix')}</p>
            <h2 className='landing-section-title'>
              {t('One gateway covering every AI scenario.')}
            </h2>
            <p className='landing-section-copy'>
              {t(
                'From your first request to stable production, one platform covers access, billing and operations.'
              )}
            </p>
          </div>
        </AnimateInView>

        <div className='mt-12 grid gap-5 md:grid-cols-2'>
          {CARDS.map((card, index) => (
            <AnimateInView delay={(index % 2) * 100} key={card.title}>
              <article
                className={cn(
                  'relative flex h-full min-h-64 flex-col overflow-hidden rounded-[1.75rem] p-8 transition-transform duration-300 hover:-translate-y-1 sm:p-10',
                  card.dark
                    ? 'bg-[#26224a] text-white shadow-[0_24px_70px_rgba(38,34,74,0.35)]'
                    : 'border border-[#101828]/8 bg-[#f6f4fe] text-[#101828] shadow-[0_16px_48px_rgba(16,24,40,0.05)]'
                )}
              >
                <h3 className='max-w-sm text-2xl font-semibold tracking-tight sm:text-3xl'>
                  {t(card.title)}
                </h3>
                <p
                  className={cn(
                    'mt-4 max-w-md text-sm leading-7 sm:text-base',
                    card.dark ? 'text-white/70' : 'text-[#667085]'
                  )}
                >
                  {t(card.copy)}
                </p>
                <div className='mt-auto pt-8'>
                  <Link
                    className={cn(
                      'inline-flex items-center gap-2 rounded-xl px-5 py-2.5 text-sm font-semibold transition-all duration-200',
                      card.dark
                        ? 'bg-gradient-to-r from-[#8b5cf6] to-[#6d28d9] text-white shadow-[0_10px_26px_rgba(124,58,237,0.4)] hover:brightness-110'
                        : 'bg-[#7c3aed] text-white shadow-[0_10px_26px_rgba(124,58,237,0.28)] hover:bg-[#6d28d9]'
                    )}
                    to={card.ctaTo ?? props.ctaTo}
                  >
                    {t(card.ctaLabel)}
                    <HugeiconsIcon
                      aria-hidden
                      data-icon='inline-end'
                      icon={ArrowRight01Icon}
                      strokeWidth={2}
                    />
                  </Link>
                </div>
                <PixelArt dark={card.dark} />
              </article>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
