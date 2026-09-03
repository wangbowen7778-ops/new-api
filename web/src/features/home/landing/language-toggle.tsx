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

import { normalizeInterfaceLanguage } from '@/i18n/languages'
import { cn } from '@/lib/utils'

/**
 * Minimal 中文/EN switcher for the fixed landing navbar.
 *
 * The site-wide `LanguageSwitcher` is a 7-language dropdown styled with semantic
 * theme tokens, which clashes with the self-contained landing palette. This
 * inline toggle keeps the two main interface languages reachable on the light
 * background while relying on `i18next-browser-languagedetector` for
 * persistence (localStorage).
 */
export function LanguageToggle() {
  const { i18n } = useTranslation()
  const current = normalizeInterfaceLanguage(i18n.language)
  const isZh = current === 'zhCN' || current === 'zhTW'

  const setLang = (code: 'zhCN' | 'en') => {
    void i18n.changeLanguage(code)
  }

  return (
    <div className='flex items-center gap-1 text-sm'>
      <button
        type='button'
        aria-pressed={isZh}
        onClick={() => setLang('zhCN')}
        className={cn(
          'rounded-md px-1.5 py-1 transition-colors duration-200',
          isZh ? 'text-[#1d1d1f]' : 'text-[#1d1d1f]/50 hover:text-[#1d1d1f]/80'
        )}
      >
        中文
      </button>
      <span aria-hidden className='text-[#1d1d1f]/30'>
        /
      </span>
      <button
        type='button'
        aria-pressed={!isZh}
        onClick={() => setLang('en')}
        className={cn(
          'rounded-md px-1.5 py-1 transition-colors duration-200',
          !isZh ? 'text-[#1d1d1f]' : 'text-[#1d1d1f]/50 hover:text-[#1d1d1f]/80'
        )}
      >
        EN
      </button>
    </div>
  )
}
