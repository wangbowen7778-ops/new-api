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

import { api } from '@/lib/api'

import {
  parseTiersFromExpr,
  splitBillingExprAndRequestRules,
} from '@/features/pricing/lib/billing-expr'

import { selectModelPreview } from './model-preview'

interface PricingBrief {
  model_name: string
  quota_type?: number
  model_ratio?: number
  completion_ratio?: number
  model_price?: number
  supported_endpoint_types?: string[]
  billing_mode?: string
  billing_expr?: string
  enable_groups?: string[]
}

interface PricingResponse {
  success?: boolean
  data?: PricingBrief[]
  supported_endpoint?: Record<string, string>
  /** Group name -> billing multiplier, e.g. { default: 1, vip: 0.1 }. */
  group_ratio?: Record<string, number>
}

/**
 * Preferred showcase order on the landing; anything else in the live pricing
 * directory fills the remaining slots. The "official" baseline is the
 * configured model price itself (ratio × $2 per 1M, or model_price per call);
 * the site price shown next to it is that price multiplied by the default
 * group's billing ratio, so the discount badge mirrors the group discount.
 */
const PREFERRED_ORDER = ['gpt-5.6-sol', 'kimi-k3', 'glm-5.3-flash'] as const

const FEATURED_LIMIT = 3

interface FeaturedModel {
  name: string
  /** Configured model price (no group discount) — per call for per-call
   *  models, per 1M input tokens for ratio-billed models. */
  officialPrice: number
  /** Configured output price per 1M tokens; null for per-call models. */
  officialOutput: number | null
  /** Site price = officialPrice × default group ratio. */
  sitePrice: number
  /** Site output price per 1M tokens; null for per-call models. */
  outputPrice: number | null
  /** Billing unit of the prices above. */
  unit: 'call' | '1M'
}

export function PricingPreview() {
  const { t } = useTranslation()
  const [models, setModels] = useState<PricingBrief[]>([])
  const [featured, setFeatured] = useState<FeaturedModel[]>([])
  const [modelCount, setModelCount] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false
    void api
      .get('/api/pricing', { skipErrorHandler: true } as Record<string, unknown>)
      .then((response) => {
        const payload = response.data as PricingResponse | undefined
        if (cancelled || !Array.isArray(payload?.data)) return
        const validModels = payload.data.filter(
          (model) => typeof model.model_name === 'string' && model.model_name.trim()
        )
        setModelCount(new Set(validModels.map((model) => model.model_name.trim())).size)
        setModels(selectModelPreview(validModels))

        // "Official" baseline = the configured model price (no group
        // discount). The site price uses the LOWEST ratio among the model's
        // enabled groups — the best price any user can get — so the discount
        // badge reads as "as low as X折".
        const groupRatios = payload.group_ratio ?? {}
        const defaultGroupRatio = groupRatios.default ?? 1

        const bestGroupRatio = (model: PricingBrief): number => {
          const ratios = (model.enable_groups ?? [])
            .map((group) => groupRatios[group])
            .filter(
              (ratio) => typeof ratio === 'number' && ratio > 0
            )
          if (ratios.length > 0) return Math.min(...ratios)
          return defaultGroupRatio
        }

        const buildFeatured = (model: PricingBrief): FeaturedModel => {
          let basePrice: number
          let outputPrice: number | null = null
          let unit: FeaturedModel['unit'] = 'call'
          if (
            model.billing_mode === 'tiered_expr' &&
            model.billing_expr &&
            (() => {
              const { billingExpr } = splitBillingExprAndRequestRules(
                model.billing_expr || ''
              )
              return parseTiersFromExpr(billingExpr).length > 0
            })()
          ) {
            // Tiered (dynamic) billing: the pricing table lives in the
            // billing expression, not in model_ratio. Show the first tier
            // (e.g. "standard"), same as the pricing page's featured tier.
            const { billingExpr } = splitBillingExprAndRequestRules(
              model.billing_expr || ''
            )
            const tier = parseTiersFromExpr(billingExpr)[0]
            const tierInput = Number(tier.inputPrice)
            const tierOutput = Number(tier.outputPrice)
            basePrice =
              Number.isFinite(tierInput) && tierInput > 0 ? tierInput : 0
            outputPrice =
              Number.isFinite(tierOutput) && tierOutput > 0 ? tierOutput : null
            unit = '1M'
          } else if (
            model.quota_type === 1 &&
            typeof model.model_price === 'number' &&
            model.model_price > 0
          ) {
            basePrice = model.model_price
            unit = 'call'
          } else {
            // Same math as the pricing page: input $/1M = model_ratio × 2,
            // output $/1M = input × completion_ratio. Missing ratios fall
            // back to the backend's runtime defaults (ratio 1).
            const ratio =
              typeof model.model_ratio === 'number' && model.model_ratio > 0
                ? model.model_ratio
                : 1
            const completionRatio =
              typeof model.completion_ratio === 'number' &&
              model.completion_ratio > 0
                ? model.completion_ratio
                : 1
            basePrice = ratio * 2
            outputPrice = basePrice * completionRatio
            unit = '1M'
          }
          return {
            name: model.model_name,
            officialPrice: basePrice,
            officialOutput: outputPrice,
            sitePrice: basePrice * bestGroupRatio(model),
            outputPrice:
              outputPrice === null
                ? null
                : outputPrice * bestGroupRatio(model),
            unit,
          }
        }

        // Prefer the curated names; fill remaining slots with any other live
        // models so the panel always shows real, current models.
        const byName = new Map(
          validModels.map((model) => [model.model_name.trim(), model])
        )
        const picked: FeaturedModel[] = []
        const used = new Set<string>()
        for (const name of PREFERRED_ORDER) {
          const model = byName.get(name)
          if (model) {
            picked.push(buildFeatured(model))
            used.add(name)
          }
          if (picked.length >= FEATURED_LIMIT) break
        }
        if (picked.length < FEATURED_LIMIT) {
          for (const model of validModels) {
            const name = model.model_name.trim()
            if (used.has(name)) continue
            picked.push(buildFeatured(model))
            used.add(name)
            if (picked.length >= FEATURED_LIMIT) break
          }
        }
        setFeatured(picked)
      })
      .catch(() => {
        // The directory remains useful as a capability overview when unavailable.
      })

    return () => {
      cancelled = true
    }
  }, [])

  const featuredWithPrice = featured.filter((item) => item.sitePrice > 0)

  return (
    <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
      <div className='mx-auto grid max-w-7xl gap-10 lg:grid-cols-[0.78fr_1.22fr] lg:items-center lg:gap-20'>
        <div>
          <p className='landing-section-kicker text-left'>{t('Live model directory')}</p>
          <h2 className='mt-4 text-4xl font-semibold tracking-[-0.04em] text-balance sm:text-5xl'>
            {t('Choose the right model without rebuilding your integration.')}
          </h2>
          <p className='mt-6 max-w-xl text-base leading-7 text-[#667085]'>
            {t(
              'Browse the model catalog and compare the pricing configured on this site. The preview is loaded from the same live directory as the pricing page.'
            )}
          </p>
          <Link className='landing-text-link mt-7' to='/pricing'>
            {t('Explore model pricing')}
            <HugeiconsIcon
              aria-hidden
              data-icon='inline-end'
              icon={ArrowRight01Icon}
              strokeWidth={2}
            />
          </Link>
        </div>

        <div className='overflow-hidden rounded-[1.75rem] border border-[#101828]/8 bg-white shadow-[0_24px_70px_rgba(16,24,40,0.08)]'>
          <div className='flex items-center justify-between gap-4 border-b border-[#101828]/8 px-5 py-5 sm:px-7'>
            <div>
              <p className='text-sm font-semibold text-[#101828]'>
                {t('Featured models')}
              </p>
              <p className='mt-1 text-xs text-[#98a2b3]'>
                {modelCount === null
                  ? t('Synced with the pricing page')
                  : t('{{count}} models currently listed', { count: modelCount })}
              </p>
            </div>
            <span className='landing-live-badge'>{t('Live')}</span>
          </div>

          <div className='min-h-64 p-4 sm:p-6'>
            {featuredWithPrice.length > 0 ? (
              <div className='flex h-full flex-col justify-center gap-5'>
                {featuredWithPrice.map((item, index) => {
                  const sitePrice = item.sitePrice ?? 0
                  const fold = (sitePrice / item.officialPrice) * 10
                  // Official price spans the full bar; the site-price overlay
                  // covers the fraction it actually costs — the gap reads as
                  // the discount.
                  const siteWidth = Math.min(
                    100,
                    Math.max(4, (sitePrice / item.officialPrice) * 100)
                  )
                  // Floor to 2 decimals (never overstate the discount) and
                  // trim trailing zeros: 1.25 → "1.25", 3 → "3".
                  const foldLabel = (
                    Math.floor(fold * 100) / 100
                  ).toString()
                  const unitSuffix =
                    item.unit === 'call' ? `/ ${t('Per request')}` : `/ ${t('1M token')}`
                  return (
                    <div className='flex flex-col gap-1' key={item.name}>
                      <div className='flex flex-wrap items-center gap-2'>
                        <span className='truncate font-mono text-xs font-semibold text-[#344054] sm:text-sm'>
                          {item.name}
                        </span>
                        {fold < 10 && (
                          <span className='rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-bold text-emerald-600 ring-1 ring-emerald-500/20'>
                            {t('{{fold}}折', { fold: foldLabel })}
                          </span>
                        )}
                      </div>
                      <div className='flex flex-wrap items-baseline gap-x-4 gap-y-0.5'>
                        <span className='text-xs text-[#98a2b3] line-through'>
                          {t('Official')} {t('Input')} $
                          {item.officialPrice.toFixed(2)} · {t('Output')} $
                          {item.officialOutput?.toFixed(2)}
                          <span className='ml-0.5'>{unitSuffix}</span>
                        </span>
                        <span className='font-mono text-sm font-bold text-[#7c3aed] sm:text-base'>
                          {t('Input')} ${sitePrice.toFixed(2)} · {t('Output')} $
                          {item.outputPrice?.toFixed(2)}
                          <span className='ml-0.5 text-xs font-medium text-[#98a2b3]'>
                            {unitSuffix}
                          </span>
                        </span>
                      </div>
                      {/* Official price as the gray baseline, site price as the
                          overlaid bar — the gap reads as the discount. */}
                      <div className='relative h-7 overflow-hidden rounded-lg bg-[#f1f4f9]'>
                        <div
                          className='landing-price-bar absolute inset-y-0 left-0 rounded-lg bg-gradient-to-r from-[#7c3aed]/30 to-[#7c3aed]'
                          style={{
                            width: `${siteWidth}%`,
                            animationDelay: `${index * 100}ms`,
                          }}
                        />
                      </div>
                    </div>
                  )
                })}
                <p className='mt-2 text-[11px] text-[#98a2b3]'>
                  {t('Real-time prices from the pricing settings.')}
                </p>
              </div>
            ) : models.length > 0 ? (
              <div className='grid gap-2 sm:grid-cols-2'>
                {models.map((model) => (
                <div
                  className='group flex min-w-0 flex-col gap-3 rounded-xl border border-[#101828]/6 bg-[#f6f8fb] px-4 py-4 transition-all duration-200 hover:-translate-y-0.5 hover:border-[#a78bfa]/30 hover:bg-white hover:shadow-[0_10px_24px_rgba(16,24,40,0.08)]'
                  key={model.model_name}
                >
                  <div className='flex min-w-0 items-center gap-3'>
                    <span className='flex size-9 shrink-0 items-center justify-center rounded-lg bg-[#efe9fe] font-mono text-xs font-bold text-[#7c3aed]'>
                      {model.model_name.slice(0, 2).toUpperCase()}
                    </span>
                    <span className='truncate font-mono text-sm font-semibold text-[#344054] sm:text-base'>
                      {model.model_name}
                    </span>
                    <span className='ml-auto shrink-0 rounded-full bg-white px-2 py-1 text-[10px] font-medium text-[#667085]'>
                      {model.quota_type === 1 ? t('Per request billing') : t('Token billing')}
                    </span>
                  </div>
                  <div className='flex min-w-0 flex-col gap-2 pl-11'>
                    {model.quota_type === 1 && typeof model.model_price === 'number' ? (
                      <span className='text-sm font-semibold whitespace-nowrap text-[#7c3aed]'>
                        ${model.model_price.toFixed(4)}
                        <span className='ml-1 text-[10px] font-medium text-[#98a2b3]'>
                          / {t('Per request')}
                        </span>
                      </span>
                    ) : (
                      <span className='text-sm font-semibold whitespace-nowrap text-[#7c3aed]'>
                        {t('Input')} ${(model.model_ratio ?? 1) * 2}/1M ·{' '}
                        {t('Output')} $
                        {(model.model_ratio ?? 1) *
                          (model.completion_ratio ?? 1) *
                          2}
                        /1M
                      </span>
                    )}
                    {(model.supported_endpoint_types ?? []).length > 0 && (
                      <div className='flex flex-wrap gap-1.5 text-[10px] font-medium text-[#98a2b3]'>
                        {(model.supported_endpoint_types ?? []).slice(0, 2).map((endpoint) => (
                          <span className='truncate' key={endpoint}>{endpoint}</span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
                ))}
              </div>
            ) : (
              <div className='flex flex-col items-center justify-center gap-3 px-6 py-12 text-center'>
                <span className='landing-catalog-mark'>AI</span>
                <p className='max-w-sm text-sm leading-6 text-[#667085]'>
                  {t(
                    'Text, reasoning, image and audio models are organized in one searchable directory.'
                  )}
                </p>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
