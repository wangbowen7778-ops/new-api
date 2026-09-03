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
import { Check, Copy } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { cn } from '@/lib/utils'

type RequestFormat = 'chat' | 'responses' | 'claude'
type CodeLanguage = 'curl' | 'python' | 'javascript' | 'java'

const FORMATS: { id: RequestFormat; label: string }[] = [
  { id: 'chat', label: 'OpenAI Completions' },
  { id: 'responses', label: 'OpenAI Responses' },
  { id: 'claude', label: 'Claude Messages' },
]

const LANGUAGES: { id: CodeLanguage; label: string }[] = [
  { id: 'curl', label: 'cURL' },
  { id: 'python', label: 'Python' },
  { id: 'javascript', label: 'JavaScript' },
  { id: 'java', label: 'Java' },
]

/** Preset models for the example; the gateway routes any of them. */
const EXAMPLE_MODELS = [
  'gpt-4o',
  'gpt-4o-mini',
  'claude-sonnet-4-5',
  'gemini-2.5-pro',
  'deepseek-v3',
  'qwen-max',
] as const

function curlSnippet(
  format: RequestFormat,
  baseUrl: string,
  model: string
): string {
  if (format === 'chat') {
    return [
      `curl ${baseUrl}/v1/chat/completions \\`,
      '  -H "Authorization: Bearer YOUR_KEY" \\',
      '  -H "Content-Type: application/json" \\',
      `  -d '{"model":"${model}","messages":[{"role":"user","content":"Hello!"}]}'`,
    ].join('\n')
  }
  if (format === 'responses') {
    return [
      `curl ${baseUrl}/v1/responses \\`,
      '  -H "Authorization: Bearer YOUR_KEY" \\',
      '  -H "Content-Type: application/json" \\',
      `  -d '{"model":"${model}","input":"Hello!"}'`,
    ].join('\n')
  }
  return [
    `curl ${baseUrl}/v1/messages \\`,
    '  -H "x-api-key: YOUR_KEY" \\',
    '  -H "anthropic-version: 2023-06-01" \\',
    '  -H "Content-Type: application/json" \\',
    `  -d '{"model":"${model}","max_tokens":1024,"messages":[{"role":"user","content":"Hello!"}]}'`,
  ].join('\n')
}

function pythonSnippet(
  format: RequestFormat,
  baseUrl: string,
  model: string
): string {
  if (format === 'chat') {
    return [
      'from openai import OpenAI',
      '',
      `client = OpenAI(base_url="${baseUrl}/v1", api_key="YOUR_KEY")`,
      '',
      'resp = client.chat.completions.create(',
      `    model="${model}",`,
      '    messages=[{"role": "user", "content": "Hello!"}],',
      ')',
      'print(resp.choices[0].message.content)',
    ].join('\n')
  }
  if (format === 'responses') {
    return [
      'from openai import OpenAI',
      '',
      `client = OpenAI(base_url="${baseUrl}/v1", api_key="YOUR_KEY")`,
      '',
      'resp = client.responses.create(',
      `    model="${model}",`,
      '    input="Hello!",',
      ')',
      'print(resp.output_text)',
    ].join('\n')
  }
  return [
    'from anthropic import Anthropic',
    '',
    `client = Anthropic(base_url="${baseUrl}", api_key="YOUR_KEY")`,
    '',
    'resp = client.messages.create(',
    `    model="${model}",`,
    '    max_tokens=1024,',
    '    messages=[{"role": "user", "content": "Hello!"}],',
    ')',
    'print(resp.content[0].text)',
  ].join('\n')
}

function javascriptSnippet(
  format: RequestFormat,
  baseUrl: string,
  model: string
): string {
  if (format === 'chat') {
    return [
      'const resp = await fetch(',
      `  "${baseUrl}/v1/chat/completions",`,
      '  {',
      '    method: "POST",',
      '    headers: {',
      '      Authorization: "Bearer YOUR_KEY",',
      '      "Content-Type": "application/json",',
      '    },',
      '    body: JSON.stringify({',
      `      model: "${model}",`,
      '      messages: [{ role: "user", content: "Hello!" }],',
      '    }),',
      '  }',
      ')',
      'const data = await resp.json()',
      'console.log(data.choices[0].message.content)',
    ].join('\n')
  }
  if (format === 'responses') {
    return [
      'const resp = await fetch(',
      `  "${baseUrl}/v1/responses",`,
      '  {',
      '    method: "POST",',
      '    headers: {',
      '      Authorization: "Bearer YOUR_KEY",',
      '      "Content-Type": "application/json",',
      '    },',
      `    body: JSON.stringify({ model: "${model}", input: "Hello!" }),`,
      '  }',
      ')',
      'const data = await resp.json()',
      'console.log(data.output_text)',
    ].join('\n')
  }
  return [
    'const resp = await fetch(',
    `  "${baseUrl}/v1/messages",`,
    '  {',
    '    method: "POST",',
    '    headers: {',
    '      "x-api-key": "YOUR_KEY",',
    '      "anthropic-version": "2023-06-01",',
    '      "Content-Type": "application/json",',
    '    },',
    '    body: JSON.stringify({',
    `      model: "${model}",`,
    '      max_tokens: 1024,',
    '      messages: [{ role: "user", content: "Hello!" }],',
    '    }),',
    '  }',
    ')',
    'const data = await resp.json()',
    'console.log(data.content[0].text)',
  ].join('\n')
}

function javaSnippet(
  format: RequestFormat,
  baseUrl: string,
  model: string
): string {
  const isClaude = format === 'claude'
  const endpoint = isClaude
    ? `${baseUrl}/v1/messages`
    : `${baseUrl}/v1/${format === 'chat' ? 'chat/completions' : 'responses'}`
  const payload = isClaude
    ? { model, max_tokens: 1024, messages: [{ role: 'user', content: 'Hello!' }] }
    : format === 'chat'
      ? { model, messages: [{ role: 'user', content: 'Hello!' }] }
      : { model, input: 'Hello!' }
  const javaBody = JSON.stringify(payload).replace(/"/g, '\\"')

  const authLines = isClaude
    ? [
        '        .header("x-api-key", "YOUR_KEY")',
        '        .header("anthropic-version", "2023-06-01")',
      ]
    : ['        .header("Authorization", "Bearer YOUR_KEY")']

  return [
    'HttpClient client = HttpClient.newHttpClient();',
    `String body = "${javaBody}";`,
    'HttpRequest request = HttpRequest.newBuilder()',
    `        .uri(URI.create("${endpoint}"))`,
    ...authLines,
    '        .header("Content-Type", "application/json")',
    '        .POST(HttpRequest.BodyPublishers.ofString(body))',
    '        .build();',
    'var response = client.send(request, HttpResponse.BodyHandlers.ofString());',
    'System.out.println(response.body());',
  ].join('\n')
}

function buildSnippet(
  format: RequestFormat,
  language: CodeLanguage,
  baseUrl: string,
  model: string
): string {
  switch (language) {
    case 'python':
      return pythonSnippet(format, baseUrl, model)
    case 'javascript':
      return javascriptSnippet(format, baseUrl, model)
    case 'java':
      return javaSnippet(format, baseUrl, model)
    default:
      return curlSnippet(format, baseUrl, model)
  }
}

/**
 * Quick start window: a light macOS-style code window with traffic-light
 * chrome, switchable request format / language / model, and a copy button.
 */
export function QuickStart() {
  const { t } = useTranslation()
  const [format, setFormat] = useState<RequestFormat>('chat')
  const [language, setLanguage] = useState<CodeLanguage>('curl')
  const [model, setModel] = useState<string>(EXAMPLE_MODELS[0])
  const [copied, setCopied] = useState<'base-url' | 'snippet' | null>(null)

  const baseUrl = typeof window !== 'undefined' ? window.location.origin : ''
  const snippet = buildSnippet(format, language, baseUrl, model)
  const snippetLines = snippet.split('\n')

  const handleCopy = async (target: 'base-url' | 'snippet', text: string) => {
    if (await copyToClipboard(text)) {
      setCopied(target)
      window.setTimeout(() => setCopied(null), 1500)
    }
  }

  const tabClass = (active: boolean) =>
    cn(
      'rounded-full px-3.5 py-1.5 text-xs font-semibold transition-all duration-200',
      active
        ? 'bg-[#7c3aed] text-white shadow-[0_4px_14px_rgba(53,111,220,0.3)]'
        : 'text-[#667085] hover:bg-[#eef2f8] hover:text-[#344054]'
    )

  return (
    <section className='px-5 py-20 sm:px-8 lg:px-10 lg:py-28'>
      <div className='mx-auto max-w-7xl'>
        <p className='landing-section-kicker text-center'>{t('Quick Start')}</p>
        <h2 className='landing-section-title mt-4 text-center'>
          {t('Three lines of code to call every model.')}
        </h2>

        <div className='mx-auto mt-12 max-w-3xl'>
          {/* Base URL */}
          <div className='overflow-hidden rounded-2xl border border-[#101828]/8 bg-white shadow-[0_16px_48px_rgba(16,24,40,0.06)]'>
            <div className='flex items-center justify-between px-5 py-3.5'>
              <span className='text-xs font-semibold text-[#667085]'>
                {t('Base URL')}
              </span>
              <button
                aria-label={t('Copy')}
                className='text-[#98a2b3] transition-colors hover:text-[#344054]'
                onClick={() => void handleCopy('base-url', baseUrl)}
                type='button'
              >
                {copied === 'base-url' ? (
                  <Check aria-hidden className='size-4 text-emerald-500' />
                ) : (
                  <Copy aria-hidden className='size-4' />
                )}
              </button>
            </div>
            <code className='block border-t border-[#101828]/6 bg-[#f8fafc] px-5 py-3.5 font-mono text-sm break-all text-[#7c3aed]'>
              {baseUrl}
            </code>
          </div>

          {/* macOS-style code window */}
          <div className='mt-4 overflow-hidden rounded-2xl border border-[#101828]/8 bg-white shadow-[0_16px_48px_rgba(16,24,40,0.08)]'>
            {/* Window chrome */}
            <div className='flex items-center gap-3 border-b border-[#101828]/6 bg-[#f6f8fb] px-4 py-3'>
              <span aria-hidden className='size-3 rounded-full bg-[#ff5f57]' />
              <span aria-hidden className='size-3 rounded-full bg-[#febc2e]' />
              <span aria-hidden className='size-3 rounded-full bg-[#28c840]' />
              <span className='flex-1 text-center font-mono text-xs text-[#98a2b3]'>
                {LANGUAGES.find((item) => item.id === language)?.label} ·{' '}
                {model}
              </span>
              <button
                aria-label={t('Copy')}
                className='text-[#98a2b3] transition-colors hover:text-[#344054]'
                onClick={() => void handleCopy('snippet', snippet)}
                type='button'
              >
                {copied === 'snippet' ? (
                  <Check aria-hidden className='size-4 text-emerald-500' />
                ) : (
                  <Copy aria-hidden className='size-4' />
                )}
              </button>
            </div>

            {/* Format tabs */}
            <div
              aria-label={t('Request format')}
              className='flex flex-wrap items-center gap-1.5 border-b border-[#101828]/6 px-4 py-3'
            >
              {FORMATS.map((item) => (
                <button
                  aria-pressed={format === item.id}
                  className={tabClass(format === item.id)}
                  key={item.id}
                  onClick={() => setFormat(item.id)}
                  type='button'
                >
                  {item.label}
                </button>
              ))}
            </div>

            {/* Language tabs + model picker */}
            <div className='flex flex-wrap items-center gap-1.5 border-b border-[#101828]/6 px-4 py-3'>
              <div
                aria-label={t('Code language')}
                className='flex flex-wrap items-center gap-1.5'
              >
                {LANGUAGES.map((item) => (
                  <button
                    aria-pressed={language === item.id}
                    className={tabClass(language === item.id)}
                    key={item.id}
                    onClick={() => setLanguage(item.id)}
                    type='button'
                  >
                    {item.label}
                  </button>
                ))}
              </div>
              <label className='ml-auto flex items-center gap-2 text-xs font-semibold text-[#667085]'>
                {t('Model')}
                <select
                  className='rounded-lg border border-[#101828]/10 bg-[#f6f8fb] px-2.5 py-1.5 font-mono text-xs text-[#344054] outline-none transition-colors focus:border-[#7c3aed]/50'
                  onChange={(event) => setModel(event.target.value)}
                  value={model}
                >
                  {EXAMPLE_MODELS.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            {/* Code with line numbers */}
            <div className='flex overflow-x-auto bg-[#f8fafc] py-4'>
              <div
                aria-hidden
                className='shrink-0 border-r border-[#101828]/6 px-3 text-right font-mono text-xs leading-relaxed text-[#98a2b3] select-none'
              >
                {snippetLines.map((_, index) => (
                  <div key={index}>{index + 1}</div>
                ))}
              </div>
              <pre className='flex-1 px-4 font-mono text-xs leading-relaxed text-[#344054] md:text-sm'>
                {snippet}
              </pre>
            </div>

            <p className='border-t border-[#101828]/6 px-5 py-3 text-xs text-[#98a2b3]'>
              {t('Replace YOUR_KEY with your API key.')}
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}
