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
import {
  AiNetworkIcon,
  ArrowRight01Icon,
  BarChartIcon,
  CodeSquareIcon,
  Key01Icon,
  Route01Icon,
  Shield01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { useSystemConfig } from '@/hooks/use-system-config'

import { Faq } from './faq'
import { HeroBanner } from './hero-banner'
import { IntegrationSteps } from './integration-steps'
import { LandingNavbar } from './navbar'
import { ModelAgentFlow } from './model-agent-flow'
import { PricingPreview } from './pricing-preview'
import { ProductMatrix } from './product-matrix'
import { QuickStart } from './quick-start'
import { StabilityMetrics } from './stability-metrics'
import { WhyChooseUs } from './why-choose-us'

interface LandingPageProps {
  isAuthenticated: boolean
}

const PROVIDER_NAMES = [
  'OpenAI',
  'Anthropic',
  'Gemini',
  'DeepSeek',
  'Qwen',
  'Grok',
  'Midjourney',
  'Suno',
] as const

export function LandingPage(props: LandingPageProps) {
  const { t } = useTranslation()
  const { systemName, footerHtml } = useSystemConfig()
  const brandName = systemName?.startsWith('云枢') ? '云枢' : systemName || '云枢'
  const ctaTo = props.isAuthenticated ? '/dashboard' : '/sign-up'
  const ctaLabel = t(props.isAuthenticated ? 'Go to Dashboard' : 'Get Started')

  return (
    <div className='landing-font-body landing-page-bg min-h-svh overflow-x-clip text-[#101828]'>
      <LandingNavbar
        brand={brandName}
        isAuthenticated={props.isAuthenticated}
      />

      <main>
        <HeroBanner ctaLabel={ctaLabel} ctaTo={ctaTo} />

        <section className='border-y border-[#101828]/8 bg-white/35 px-5 py-7 sm:px-8 lg:px-10'>
          <p className='text-center text-[11px] font-semibold tracking-[0.2em] text-[#98a2b3] uppercase'>
            {t('Connect leading model ecosystems through one gateway')}
          </p>
          <div className='mt-5 flex flex-wrap items-center justify-center gap-x-8 gap-y-4 text-sm font-semibold tracking-tight text-[#475467] sm:gap-x-12'>
            {PROVIDER_NAMES.map((name) => (
              <span key={name}>{name}</span>
            ))}
          </div>
        </section>

        <section className='px-5 py-16 sm:px-8 lg:px-10 lg:py-20'>
          <div className='mx-auto max-w-7xl'>
            <ModelAgentFlow brand={brandName} />
          </div>
        </section>

        <ProductMatrix ctaTo={ctaTo} />

        <IntegrationSteps isAuthenticated={props.isAuthenticated} />

        <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
          <div className='mx-auto max-w-7xl'>
            <AnimateInView>
              <div className='mx-auto max-w-3xl text-center'>
                <p className='landing-section-kicker'>{t('Platform capabilities')}</p>
                <h2 className='landing-section-title'>
                  {t('Everything your AI products need to move faster.')}
                </h2>
                <p className='landing-section-copy'>
                  {t(
                    'Replace fragmented provider integrations with a single operational layer for access, routing, observability and cost control.'
                  )}
                </p>
              </div>
            </AnimateInView>

            <div className='mt-12 grid gap-4 md:grid-cols-2 lg:grid-cols-4'>
              {[
                {
                  icon: AiNetworkIcon,
                  title: 'Unified model access',
                  copy: 'Use familiar API formats to reach text, reasoning, image and audio capabilities.',
                },
                {
                  icon: Route01Icon,
                  title: 'Flexible routing',
                  copy: 'Manage channels and model mappings from one console as your provider mix evolves.',
                },
                {
                  icon: BarChartIcon,
                  title: 'Usage visibility',
                  copy: 'Review request records, token usage and spend without stitching together provider dashboards.',
                },
                {
                  icon: Shield01Icon,
                  title: 'Access governance',
                  copy: 'Issue scoped keys and manage users, groups and permissions for team-ready operations.',
                },
              ].map((item, index) => (
                <AnimateInView delay={index * 80} key={item.title}>
                  <article className='landing-capability-card'>
                    <span className='landing-icon-box'>
                      <HugeiconsIcon
                        aria-hidden
                        icon={item.icon}
                        strokeWidth={1.7}
                      />
                    </span>
                    <h3>{t(item.title)}</h3>
                    <p>{t(item.copy)}</p>
                  </article>
                </AnimateInView>
              ))}
            </div>
          </div>
        </section>

        <PricingPreview />

        <WhyChooseUs />

        <StabilityMetrics />

        <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
          <div className='mx-auto grid max-w-7xl gap-12 rounded-[2rem] border border-[#101828]/8 bg-white/75 p-7 shadow-[0_24px_80px_rgba(16,24,40,0.06)] backdrop-blur md:p-12 lg:grid-cols-[0.8fr_1.2fr] lg:items-center lg:p-16'>
            <AnimateInView>
              <p className='landing-section-kicker text-left'>{t('Built for delivery')}</p>
              <h2 className='mt-4 text-3xl font-semibold tracking-[-0.035em] text-balance sm:text-4xl'>
                {t('From first request to production operations.')}
              </h2>
              <p className='mt-5 text-base leading-7 text-[#667085]'>
                {t(
                  'Start with an OpenAI-compatible endpoint, then grow into model management, team access and detailed usage reporting without changing platforms.'
                )}
              </p>
            </AnimateInView>
            <div className='grid gap-3 sm:grid-cols-2'>
              {[
                { icon: Key01Icon, label: 'Create a scoped API key' },
                { icon: CodeSquareIcon, label: 'Switch the base URL in your SDK' },
                { icon: Route01Icon, label: 'Choose models and routing policies' },
                { icon: BarChartIcon, label: 'Track requests and usage in one place' },
              ].map((item, index) => (
                <AnimateInView delay={index * 70} key={item.label}>
                  <div className='flex h-full items-start gap-4 rounded-2xl bg-[#f6f8fb] p-5'>
                    <HugeiconsIcon
                      aria-hidden
                      className='mt-0.5 size-5 shrink-0 text-[#7c3aed]'
                      icon={item.icon}
                      strokeWidth={1.8}
                    />
                    <span className='text-sm font-medium leading-6 text-[#344054]'>
                      {t(item.label)}
                    </span>
                  </div>
                </AnimateInView>
              ))}
            </div>
          </div>
        </section>

        <QuickStart />
        <Faq />

        <section className='px-5 pb-24 sm:px-8 lg:px-10 lg:pb-32'>
          <AnimateInView animation='scale-in'>
            <div className='landing-cta-panel mx-auto max-w-7xl overflow-hidden rounded-[2rem] px-7 py-16 text-center text-white sm:px-12 lg:py-24'>
              <p className='text-xs font-semibold tracking-[0.18em] text-white/55 uppercase'>
                {brandName} · AI Gateway
              </p>
              <h2 className='landing-font-instrument mx-auto mt-5 max-w-4xl text-5xl leading-[0.98] tracking-[-0.035em] text-balance sm:text-6xl lg:text-7xl'>
                {t('One gateway for the models that power your next product.')}
              </h2>
              <Link className='landing-btn-light mt-9' to={ctaTo}>
                {ctaLabel}
                <HugeiconsIcon
                  aria-hidden
                  data-icon='inline-end'
                  icon={ArrowRight01Icon}
                  strokeWidth={2}
                />
              </Link>
            </div>
          </AnimateInView>
        </section>
      </main>

      <footer className='border-t border-[#101828]/8 px-5 py-8 sm:px-8 lg:px-10'>
        <div className='mx-auto flex max-w-7xl flex-col items-center gap-5 sm:flex-row sm:justify-between'>
          <span className='landing-font-brand text-lg text-[#101828]'>
            {brandName}
          </span>
          <nav className='flex flex-wrap items-center justify-center gap-x-6 gap-y-3 text-sm text-[#667085]'>
            <Link className='transition-colors hover:text-[#101828]' to='/pricing'>
              {t('Pricing')}
            </Link>
            <Link className='transition-colors hover:text-[#101828]' to='/docs'>
              {t('Docs')}
            </Link>
            <Link className='transition-colors hover:text-[#101828]' to='/about'>
              {t('About')}
            </Link>
          </nav>
        </div>
        {footerHtml && (
          <div
            className='mx-auto mt-5 max-w-7xl text-center text-xs text-[#98a2b3]'
            dangerouslySetInnerHTML={{ __html: footerHtml }}
          />
        )}
      </footer>
    </div>
  )
}
