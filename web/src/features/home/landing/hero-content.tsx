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
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

interface HeroContentProps {
  isAuthenticated: boolean
}

export function HeroContent({ isAuthenticated }: HeroContentProps) {
  const { t } = useTranslation()

  return (
    <div className='absolute inset-0 -mt-[72px] flex items-center justify-center px-6 text-center md:-mt-[120px]'>
      <div>
        <h1 className='landing-font-instrument text-glow text-[36px] leading-[0.9] tracking-tight text-white md:text-7xl lg:text-[110px]'>
          <span>{t('One Gateway.')}</span>
          <br />
          <span>{t('Every Model.')}</span>
        </h1>
        <p className='mx-auto mt-5 text-sm text-white/70 md:mt-7 md:text-base'>
          {t(
            'One key, every model. Unified access to hundreds of top AI models via a single API, with transparent billing and round-the-clock stability.'
          )}
        </p>
        <Link
          to={isAuthenticated ? '/dashboard' : '/sign-up'}
          className={cn(
            'landing-btn-primary button-glow mt-6 inline-block md:mt-9',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/60',
            'focus-visible:ring-offset-2 focus-visible:ring-offset-[#0a0608]'
          )}
        >
          {t(isAuthenticated ? 'Go to Dashboard' : 'Get Started')}
        </Link>
      </div>
    </div>
  )
}
