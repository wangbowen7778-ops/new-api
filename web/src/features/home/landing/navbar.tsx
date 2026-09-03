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
import { Link } from '@tanstack/react-router'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { LanguageToggle } from './language-toggle'

const menuEase = 'ease-[cubic-bezier(0.22,1,0.36,1)]'

interface LandingNavbarProps {
  brand: string
  isAuthenticated: boolean
}

interface NavLinkItem {
  label: string
  to: '/pricing' | '/docs' | '/about'
}

export function LandingNavbar({ brand, isAuthenticated }: LandingNavbarProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const close = useCallback(() => setOpen(false), [])

  // Lock body scroll while the mobile panel is open and close it on Escape.
  useEffect(() => {
    if (!open) return
    document.body.style.overflow = 'hidden'
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false)
    }
    window.addEventListener('keydown', onKey)
    return () => {
      document.body.style.overflow = ''
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  const centerLinks: NavLinkItem[] = [
    { label: t('Pricing'), to: '/pricing' },
    { label: t('Docs'), to: '/docs' },
    { label: t('About'), to: '/about' },
  ]

  const cta = {
    label: t(isAuthenticated ? 'Go to Dashboard' : 'Get Started'),
    to: isAuthenticated ? ('/dashboard' as const) : ('/sign-up' as const),
  }

  return (
    <>
      <header className='fixed inset-x-0 top-0 z-50 flex min-h-18 items-center justify-between border-b border-[#101828]/7 bg-[#f8faff]/80 px-5 backdrop-blur-xl sm:px-8 md:grid md:grid-cols-[1fr_auto_1fr] lg:px-10'>
        {/* Left column — the configured system name as the brand */}
        <Link
          to='/'
          className='landing-font-brand justify-self-start text-lg text-[#101828]'
        >
          {brand}
        </Link>

        {/* Desktop center nav — the middle `auto` grid column keeps it centered
         * on the viewport regardless of brand / right-group widths. */}
        <nav className='hidden items-center gap-9 md:flex'>
          {centerLinks.map((item) => (
            <Link
              key={item.label}
              to={item.to}
              className='text-sm font-medium text-[#667085] transition-colors duration-200 hover:text-[#101828]'
            >
              {item.label}
            </Link>
          ))}
        </nav>

        <div className='hidden items-center justify-self-end gap-4 md:flex'>
          <LanguageToggle />
          <Link className='landing-nav-cta' to={cta.to}>
            {cta.label}
          </Link>
        </div>

        {/* Mobile right */}
        <div className='flex items-center gap-3 md:hidden'>
          <LanguageToggle />
          <button
            type='button'
            onClick={() => setOpen((value) => !value)}
            aria-expanded={open}
            aria-controls='landing-mobile-menu'
            aria-label={t('Toggle navigation menu')}
            className='flex size-9 items-center justify-center'
          >
            <div className='relative size-4'>
              <span
                className={cn(
                  'absolute inset-x-0 top-[1px] h-[1.5px] rounded-full bg-[#1d1d1f] transition-all duration-300',
                  menuEase,
                  open ? 'translate-y-[9px] rotate-45' : ''
                )}
              />
              <span
                className={cn(
                  'absolute inset-x-0 top-[7px] h-[1.5px] rounded-full bg-[#1d1d1f] transition-all duration-300',
                  open ? 'scale-0 opacity-0' : ''
                )}
              />
              <span
                className={cn(
                  'absolute inset-x-0 top-[13px] h-[1.5px] rounded-full bg-[#1d1d1f] transition-all duration-300',
                  menuEase,
                  open ? '-translate-y-[9px] -rotate-45' : ''
                )}
              />
            </div>
          </button>
        </div>
      </header>

      {/* Mobile slide-in panel */}
      <div
        id='landing-mobile-menu'
        className={cn(
          'fixed inset-0 z-40 md:hidden',
          open ? 'pointer-events-auto' : 'pointer-events-none'
        )}
      >
        <div
          aria-hidden
          onClick={close}
          className={cn(
            'absolute inset-0 bg-black/20 transition-opacity duration-300',
            open ? 'opacity-100' : 'opacity-0'
          )}
        />
        <div
          className={cn(
            'absolute top-0 right-0 h-full w-[85%] max-w-[340px] border-l border-[#101828]/7 bg-[#f8faff]/97 backdrop-blur-xl',
            'transition-transform duration-300',
            menuEase,
            open ? 'translate-x-0' : 'translate-x-full'
          )}
        >
          <nav className='flex flex-col gap-2 px-8 pt-24'>
            {centerLinks.map((item, index) => (
              <Link
                key={item.label}
                to={item.to}
                onClick={close}
                className='text-lg text-[#1d1d1f]/85 transition-all duration-300 hover:text-[#1d1d1f]'
                style={{
                  transitionDelay: open ? `${150 + index * 75}ms` : '0ms',
                  opacity: open ? 1 : 0,
                  transform: open ? 'translateX(0)' : 'translateX(16px)',
                }}
              >
                {item.label}
              </Link>
            ))}
          </nav>
          <div className='px-8 pt-10'>
            <Link
              to={cta.to}
              onClick={close}
              className='landing-btn-primary block w-full text-center focus-visible:ring-2 focus-visible:ring-[#0071e3]/60 focus-visible:ring-offset-2 focus-visible:ring-offset-[#f5f5f7] focus-visible:outline-none'
            >
              {cta.label}
            </Link>
          </div>
        </div>
      </div>
    </>
  )
}
