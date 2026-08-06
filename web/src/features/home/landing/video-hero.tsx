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
import { useSystemConfig } from '@/hooks/use-system-config'

import { LANDING_VIDEO_URL } from './constants'
import { HeroContent } from './hero-content'
import { LandingNavbar } from './navbar'

interface VideoHeroLandingProps {
  isAuthenticated: boolean
}

/**
 * Single full-screen video hero landing page for the token relay station.
 *
 * Always-dark by design: every color is a hardcoded hex / white-alpha value so
 * the section stays consistent regardless of the app's light/dark theme.
 */
export function VideoHeroLanding({ isAuthenticated }: VideoHeroLandingProps) {
  const { systemName, footerHtml } = useSystemConfig()

  return (
    <div className='relative min-h-svh overflow-hidden bg-[#0a0608] text-white'>
      {/* Fallback gradient — keeps the screen intentional if the video fails */}
      <div
        aria-hidden
        className='absolute inset-0 bg-[radial-gradient(ellipse_80%_60%_at_50%_35%,rgba(255,255,255,0.07)_0%,transparent_60%)]'
      />
      {/* Background video */}
      <video
        className='absolute inset-0 h-full w-full object-cover'
        src={LANDING_VIDEO_URL}
        autoPlay
        muted
        loop
        playsInline
        preload='metadata'
        aria-hidden
      />
      {/* Darkening overlay */}
      <div aria-hidden className='absolute inset-0 bg-black/20' />

      <LandingNavbar brand={systemName} isAuthenticated={isAuthenticated} />
      <HeroContent isAuthenticated={isAuthenticated} />

      {/* ICP filing number (备案号) from the Footer option */}
      {footerHtml && (
        <div className='absolute inset-x-0 bottom-3 flex justify-center px-6'>
          <div
            className='text-[11px] opacity-70'
            dangerouslySetInnerHTML={{ __html: footerHtml }}
          />
        </div>
      )}
    </div>
  )
}
