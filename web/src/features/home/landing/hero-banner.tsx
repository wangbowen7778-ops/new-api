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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

interface HeroBannerProps {
  ctaTo: '/dashboard' | '/sign-up'
  ctaLabel: string
}

interface HeroSlide {
  titleBase: string
  titleAccent: string
  subtitle: string
  ctaLabel: string
  ctaTo: '/pricing' | null
}

const SLIDES: HeroSlide[] = [
  {
    titleBase: 'One gateway for ',
    titleAccent: 'every AI model.',
    subtitle:
      'Unified access to chat, reasoning, image and audio models with one key and one bill.',
    ctaLabel: 'Get Started',
    ctaTo: null,
  },
  {
    titleBase: 'Stable relay, ',
    titleAccent: 'around the clock.',
    subtitle:
      'Multi-channel failover and intelligent routing keep every request on the fastest healthy route, around the clock.',
    ctaLabel: 'Get Started',
    ctaTo: null,
  },
  {
    titleBase: 'Transparent pricing, ',
    titleAccent: 'pay as you go.',
    subtitle:
      'Pay only for what you use, with real-time usage logs and per-model pricing you can audit at any time.',
    ctaLabel: 'View Pricing',
    ctaTo: '/pricing',
  },
]

const SLIDE_INTERVAL_MS = 6000

/**
 * Each banner slide carries its own signature backdrop and the layers
 * crossfade together with the slide content:
 *   1. violet aurora — drifting color pools
 *   2. blue ribbons — sweeping bezier curves
 *   3. warm beams — angled translucent bars rising on the right
 */
function SlideBackdrops(props: { active: number }) {
  const layerClass = (index: number) =>
    cn(
      'absolute inset-0 transition-opacity duration-700',
      index === props.active ? 'opacity-100' : 'opacity-0'
    )

  return (
    <>
      {/* Slide 1: violet aurora + geometric accents */}
      <div
        aria-hidden
        className={cn(
          layerClass(0),
          'bg-[linear-gradient(180deg,#f8f6ff_0%,#f6f4fd_48%,#faf8fe_100%)]'
        )}
      >
        <div className='landing-orb absolute -top-24 left-[4%] bg-[rgb(168_85_247_/_0.26)]' />
        <div className='landing-orb absolute top-[30%] right-[-10rem] bg-[rgb(96_165_250_/_0.22)]' />
        <div className='landing-orb absolute bottom-[-12rem] left-[38%] bg-[rgb(232_121_249_/_0.18)]' />
        {/* Outlined rounded squares, top right */}
        <svg
          className='absolute top-[12%] right-[7%] hidden h-64 w-64 text-[#8b5cf6]/30 md:block'
          fill='none'
          stroke='currentColor'
          strokeWidth='1.5'
          viewBox='0 0 240 240'
        >
          <rect height='140' rx='24' width='140' x='20' y='20' />
          <rect height='140' rx='24' width='140' x='72' y='72' opacity='0.55' />
          <circle cx='186' cy='42' r='6' fill='currentColor' stroke='none' />
        </svg>
        {/* Pixel mosaic, bottom right */}
        <div className='absolute right-[10%] bottom-[16%] grid grid-cols-4 gap-2'>
          {[1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 1, 1, 0, 1, 0, 0].map(
            (cell, index) => (
              <span
                className={cn(
                  'size-5 rounded-md',
                  cell === 1 ? 'bg-[#7c3aed]/14' : 'bg-[#7c3aed]/5'
                )}
                key={index}
              />
            )
          )}
        </div>
        {/* Sparkle marks, left */}
        <svg
          aria-hidden
          className='absolute bottom-[22%] left-[6%] hidden h-16 w-16 text-[#a855f7]/35 lg:block'
          fill='none'
          stroke='currentColor'
          strokeLinecap='round'
          strokeWidth='1.5'
          viewBox='0 0 64 64'
        >
          <path d='M32 8 v16 M24 16 h16' />
          <path d='M50 34 v12 M44 40 h12' />
        </svg>
      </div>

      {/* Slide 2: blue ribbons */}
      <div
        aria-hidden
        className={cn(
          layerClass(1),
          'bg-[linear-gradient(180deg,#f0f7ff_0%,#edf3fd_48%,#f3f7ff_100%)]'
        )}
      >
        <div className='landing-orb absolute top-[8%] right-[8%] bg-[rgb(59_130_246_/_0.18)]' />
        <svg
          className='absolute inset-0 h-full w-full'
          preserveAspectRatio='none'
          viewBox='0 0 1440 800'
        >
          <defs>
            <linearGradient id='hero-ribbon' x1='0' x2='1' y1='0' y2='0'>
              <stop offset='0' stopColor='#7c3aed' stopOpacity='0' />
              <stop offset='0.4' stopColor='#8b5cf6' stopOpacity='0.5' />
              <stop offset='0.75' stopColor='#60a5fa' stopOpacity='0.5' />
              <stop offset='1' stopColor='#60a5fa' stopOpacity='0' />
            </linearGradient>
          </defs>
          <path
            d='M-100 640 C 400 440 900 780 1540 300'
            fill='none'
            stroke='url(#hero-ribbon)'
            strokeWidth='1.6'
          />
          <path
            d='M-100 700 C 420 520 920 820 1540 380'
            fill='none'
            opacity='0.55'
            stroke='url(#hero-ribbon)'
            strokeWidth='1.2'
          />
          <path
            d='M-100 560 C 380 360 880 700 1540 220'
            fill='none'
            opacity='0.3'
            stroke='url(#hero-ribbon)'
            strokeWidth='2'
          />
        </svg>
      </div>

      {/* Slide 3: warm beams */}
      <div
        aria-hidden
        className={cn(
          layerClass(2),
          'bg-[linear-gradient(180deg,#fff8f4_0%,#fdf4f6_48%,#fef7f5_100%)]'
        )}
      >
        <div className='landing-orb absolute top-[16%] right-[6%] bg-[rgb(244_114_182_/_0.16)]' />
        {/* Angled translucent bars drifting upward on the right side */}
        <div className='absolute right-[6%] bottom-0 flex h-[70%] items-end gap-5'>
          {[
            { height: '55%', tone: 'from-[#f472b6]/20 to-[#f472b6]/5' },
            { height: '85%', tone: 'from-[#a855f7]/22 to-[#a855f7]/6' },
            { height: '65%', tone: 'from-[#fb923c]/18 to-[#fb923c]/4' },
            { height: '95%', tone: 'from-[#f472b6]/16 to-[#a855f7]/6' },
          ].map((bar, index) => (
            <div
              className={`w-10 rounded-t-2xl bg-gradient-to-b ${bar.tone} md:w-14`}
              key={index}
              style={{ height: bar.height }}
            />
          ))}
        </div>
      </div>
    </>
  )
}

/**
 * Full-viewport hero banner carousel: left-aligned headline with a purple
 * accent tail, soft lavender background with drifting color blobs and a
 * ribbon curve, crossfading slides with dash indicators.
 */
export function HeroBanner(props: HeroBannerProps) {
  const { t } = useTranslation()
  const [active, setActive] = useState(0)

  useEffect(() => {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
    const timer = window.setInterval(() => {
      setActive((index) => (index + 1) % SLIDES.length)
    }, SLIDE_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [])

  return (
    <section className='relative isolate flex min-h-svh flex-col overflow-hidden'>
      {/* Shared grid + per-slide signature backdrops that crossfade */}
      <div aria-hidden className='pointer-events-none absolute inset-0 -z-10'>
        <div className='landing-grid-bg absolute inset-0' />
        <SlideBackdrops active={active} />
      </div>

      <div className='relative mx-auto flex w-full max-w-7xl flex-1 flex-col justify-center px-5 pt-24 pb-20 sm:px-8 lg:px-10'>
        {/* Eyebrow stays fixed while slides crossfade */}
        <div className='landing-eyebrow mb-8'>
          <span aria-hidden className='landing-live-dot' />
          {t('Multi-protocol compatible, as low as 90% off')}
        </div>

        <div className='grid'>
          {SLIDES.map((slide, index) => {
            const isActive = index === active
            return (
              <div
                aria-hidden={!isActive}
                className={cn(
                  'col-start-1 row-start-1 transition-opacity duration-700',
                  isActive ? 'opacity-100' : 'pointer-events-none opacity-0'
                )}
                key={slide.titleBase}
              >
                <h1 className='text-4xl leading-[1.08] font-semibold tracking-[-0.03em] text-balance text-[#101828] sm:text-5xl lg:text-6xl'>
                  {t(slide.titleBase)}
                  <span className='landing-gradient-text'>
                    {t(slide.titleAccent)}
                  </span>
                </h1>
                <p className='mt-6 max-w-xl text-base leading-8 text-[#667085] sm:text-lg'>
                  {t(slide.subtitle)}
                </p>
                <div className='mt-9 flex flex-wrap items-center gap-3'>
                  <Link
                    className='landing-btn-primary'
                    to={slide.ctaTo ?? props.ctaTo}
                  >
                    {t(slide.ctaLabel)}
                    <HugeiconsIcon
                      aria-hidden
                      data-icon='inline-end'
                      icon={ArrowRight01Icon}
                      strokeWidth={2}
                    />
                  </Link>
                  <Link className='landing-btn-secondary' to='/docs'>
                    {t('View documentation')}
                  </Link>
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Dash indicators */}
      <div className='absolute inset-x-0 bottom-10 flex justify-center gap-2.5'>
        {SLIDES.map((slide, index) => (
          <button
            aria-label={t('Slide {{index}}', { index: index + 1 })}
            className={cn(
              'h-1 rounded-full transition-all duration-300',
              index === active
                ? 'w-8 bg-[#7c3aed]'
                : 'w-4 bg-[#101828]/15 hover:bg-[#101828]/30'
            )}
            key={slide.titleBase}
            onClick={() => setActive(index)}
            type='button'
          />
        ))}
      </div>
    </section>
  )
}
