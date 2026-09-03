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
import { AiBrain01Icon, AiNetworkIcon, CodeSquareIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

interface ModelAgentFlowProps {
  brand: string
}

const SOURCES = ['Claude', 'OpenAI', 'Gemini', 'GLM'] as const
const DESTINATIONS = ['Claude Code', 'Codex', 'OpenClaw', 'Zcode'] as const

/** Pastel color per lane, so each route reads as its own thread. */
const LANE_COLORS = ['#7c3aed', '#7357c7', '#12b76a', '#f59e0b'] as const

/** Vertical positions of the flow lanes inside the 260x280 viewBox. */
const FLOW_PATH_Y = [55, 118, 181, 244] as const

function lanePath(y: number): string {
  return `M0 ${y} C70 ${y} 92 140 130 140 S190 ${y} 260 ${y}`
}

/**
 * Open data-flow canvas: model providers on the left, agent clients on the
 * right, colored curves converging through the gateway hub in the middle.
 * Each lane carries a moving particle along its curve; reduced-motion hides
 * the particles and freezes the dash flow.
 */
export function ModelAgentFlow(props: ModelAgentFlowProps) {
  const { t } = useTranslation()

  const renderNode = (name: string, isSource: boolean) => (
    <div className='landing-flow-node' key={name}>
      <span
        className={
          isSource
            ? 'landing-flow-node-icon'
            : 'landing-flow-node-icon landing-flow-node-icon-output'
        }
      >
        <HugeiconsIcon
          aria-hidden
          icon={isSource ? AiBrain01Icon : CodeSquareIcon}
          strokeWidth={1.7}
        />
      </span>
      <span>{name}</span>
    </div>
  )

  return (
    <div className='landing-flow-canvas relative mx-auto w-full'>
      <div className='landing-flow-stage'>
        <div className='landing-flow-column'>
          <p className='landing-flow-label'>{t('Model')}</p>
          {SOURCES.map((name) => renderNode(name, true))}
        </div>

        <div aria-hidden className='landing-flow-center'>
          <svg
            className='landing-flow-lines'
            preserveAspectRatio='none'
            viewBox='0 0 260 280'
          >
            {FLOW_PATH_Y.map((y, index) => {
              const path = lanePath(y)
              const color = LANE_COLORS[index % LANE_COLORS.length]
              return (
                <g key={y}>
                  <path
                    className='landing-flow-glowline'
                    d={path}
                    style={{ stroke: color }}
                  />
                  <path
                    className='landing-flow-line'
                    d={path}
                    style={{
                      stroke: color,
                      animationDelay: `${index * 0.4}s`,
                    }}
                  />
                  {/* Particles ride only on alternate lanes so the canvas
                      stays calm instead of busy. */}
                  {index % 2 === 0 && (
                    <circle
                      className='landing-flow-particle'
                      r='2.5'
                      style={{ fill: color }}
                    >
                      <animateMotion
                        begin={`${index * 1.2}s`}
                        dur='4.8s'
                        path={path}
                        repeatCount='indefinite'
                      />
                    </circle>
                  )}
                </g>
              )
            })}
          </svg>
          <div className='flex flex-col items-center'>
            <div className='landing-flow-hub'>
              <HugeiconsIcon
                aria-hidden
                icon={AiNetworkIcon}
                strokeWidth={1.5}
              />
            </div>
            <span className='landing-flow-hub-caption'>
              <i aria-hidden />
              {props.brand} · {t('Route & govern')}
            </span>
          </div>
        </div>

        <div className='landing-flow-column'>
          <p className='landing-flow-label'>{t('Agent workflow')}</p>
          {DESTINATIONS.map((name) => renderNode(name, false))}
        </div>
      </div>
    </div>
  )
}
