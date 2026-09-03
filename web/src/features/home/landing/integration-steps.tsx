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
  AiUserIcon,
  BookOpen01Icon,
  Key01Icon,
  PaymentIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

interface IntegrationStepsProps {
  isAuthenticated: boolean
}

const STEPS = [
  {
    number: '01',
    icon: AiUserIcon,
    title: 'Register or sign in',
    description: 'Create your Yunshu account and open the console to get started.',
    action: 'Register / sign in',
    to: '/sign-up' as const,
  },
  {
    number: '02',
    icon: PaymentIcon,
    title: 'Add available balance',
    description: 'Choose a top-up option and prepare balance for model requests.',
    action: 'View pricing',
    to: '/pricing' as const,
  },
  {
    number: '03',
    icon: Key01Icon,
    title: 'Create an API key',
    description: 'Issue a scoped key in the console for secure service authentication.',
    action: 'Create API key',
    to: '/keys' as const,
  },
  {
    number: '04',
    icon: BookOpen01Icon,
    title: 'Call the API',
    description: 'Follow the integration guide and connect your application in minutes.',
    action: 'Read the docs',
    to: '/docs' as const,
  },
] as const

export function IntegrationSteps(props: IntegrationStepsProps) {
  const { t } = useTranslation()

  return (
    <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
      <div className='mx-auto max-w-7xl'>
        <div className='mx-auto max-w-3xl text-center'>
          <p className='landing-section-kicker'>{t('Four steps to connect')}</p>
          <h2 className='landing-section-title'>{t('Complete your AI integration with confidence.')}</h2>
          <p className='landing-section-copy'>
            {t('From account setup to your first request, every step is clear and measurable.')}
          </p>
        </div>

        <div className='landing-step-grid mt-12 grid gap-4 md:grid-cols-2 lg:grid-cols-4'>
          {STEPS.map((step, index) => {
            const isCurrent = index === 0 && !props.isAuthenticated
            return (
              <article className='landing-step-card' key={step.number}>
                <div className='flex items-center justify-between'>
                  <span className='landing-step-number'>{step.number}</span>
                  <HugeiconsIcon aria-hidden className='size-6 text-[#7c3aed]' icon={step.icon} strokeWidth={1.7} />
                </div>
                <h3>{t(step.title)}</h3>
                <p>{t(step.description)}</p>
                <Link className='landing-step-link' to={isCurrent ? step.to : step.to}>
                  {t(step.action)}
                  <span aria-hidden>↗</span>
                </Link>
              </article>
            )
          })}
        </div>
      </div>
    </section>
  )
}
