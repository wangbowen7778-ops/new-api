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
import { ArrowDown01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

interface FaqItem {
  question: string
  answer: string
}

/** FAQ accordion built on native details/summary for keyboard accessibility. */
export function Faq() {
  const { t } = useTranslation()

  const items: FaqItem[] = [
    {
      question: t('How am I billed?'),
      answer: t(
        'You only pay for what you use: token-based models are billed by usage, per-call models by request. Every call is itemized in your usage logs.'
      ),
    },
    {
      question: t('Do I need a subscription?'),
      answer: t(
        'Account balance, top-up options and billing rules are shown in your console. Review the pricing page before sending production traffic.'
      ),
    },
    {
      question: t('Which models and protocols are supported?'),
      answer: t(
        'Open the live pricing page to see the models and endpoint types currently configured on this site.'
      ),
    },
    {
      question: t('How do I integrate an existing application?'),
      answer: t(
        'Keep the SDK your application already uses, change its base URL to this gateway and authenticate with a key created in your console.'
      ),
    },
  ]

  return (
    <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
      <div className='mx-auto max-w-4xl'>
        <p className='landing-section-kicker'>{t('Questions')}</p>
        <h2 className='landing-section-title text-center'>
          {t('FAQ')}
        </h2>
        <div className='mt-10 flex flex-col gap-3'>
          {items.map((item) => (
            <details
              className='group rounded-2xl border border-[#101828]/8 bg-white/80 px-5 py-5 shadow-[0_12px_40px_rgba(16,24,40,0.04)]'
              key={item.question}
            >
              <summary className='flex cursor-pointer list-none items-center justify-between gap-4 text-sm font-semibold text-[#101828] md:text-base [&::-webkit-details-marker]:hidden'>
                {item.question}
                <HugeiconsIcon
                  aria-hidden
                  className='size-4 shrink-0 text-[#6e6e73] transition-transform duration-300 group-open:rotate-180'
                  icon={ArrowDown01Icon}
                  strokeWidth={2}
                />
              </summary>
              <p className='mt-3 text-sm leading-7 text-[#667085]'>
                {item.answer}
              </p>
            </details>
          ))}
        </div>
      </div>
    </section>
  )
}
