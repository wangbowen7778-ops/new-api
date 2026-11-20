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
import { useQuery } from '@tanstack/react-query'
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

// Static doc files live under web/public/docs-content (served at
// /docs-content/...). The name avoids colliding with the /docs SPA route —
// a dist/docs/ directory would make the Go static server redirect
// /docs <-> /docs/ in a loop instead of serving the page.
const DOCS_MD_URL = '/docs-content/ShwllAPI使用文档.md'
const DOCS_BASE_PATH = '/docs-content/'

type TocNode = { id: string; text: string; children: TocNode[] }

async function fetchDocsContent(): Promise<string> {
  // 'no-store' bypasses the backend's one-week static Cache-Control header so
  // a rebuilt document is picked up immediately.
  const res = await fetch(DOCS_MD_URL, { cache: 'no-store' })
  if (!res.ok) {
    throw new Error(`Docs request failed: ${res.status} ${res.statusText}`)
  }
  return res.text()
}

/**
 * Rewrite relative image refs (`![alt](img_x.png)`) to absolute
 * `/docs-content/img_x.png` so they resolve regardless of the route trailing
 * slash (`/docs` vs `/docs/`). Absolute URLs, root-relative paths, and
 * in-page anchors are left untouched.
 */
function rewriteDocImageUrls(markdown: string): string {
  return markdown.replaceAll(
    /!\[([^\]]*)\]\(([^)]+)\)/g,
    (match, alt: string, targetRaw: string) => {
      const target = targetRaw.trim()
      const url = target.split(/\s+/)[0] ?? ''
      if (/^(?:[a-z][a-z0-9+.-]*:|\/|#)/i.test(url)) {
        return match
      }
      const rest = target.slice(url.length) // preserve optional "title" suffix
      return `![${alt}](${DOCS_BASE_PATH}${url}${rest})`
    }
  )
}

export function DocsPage() {
  const { t } = useTranslation()
  const contentRef = useRef<HTMLDivElement>(null)
  // Marks where the doc content begins below the fixed header — used to compute
  // precise scroll targets for TOC clicks regardless of the header height.
  const layoutRef = useRef<HTMLDivElement>(null)
  // heading id -> h2 section id (pure string map, survives Markdown re-renders).
  const idToSectionRef = useRef<Map<string, string>>(new Map())
  const [toc, setToc] = useState<TocNode[]>([])
  // The exact heading under the read position (for highlighting sub-items).
  const [activeId, setActiveId] = useState<string | null>(null)
  // The h2 section owning the active heading (for expanding sub-items).
  const [activeSectionId, setActiveSectionId] = useState<string | null>(null)

  const { data, isLoading, isError, refetch } = useQuery({
    queryKey: ['docs', 'content'],
    queryFn: fetchDocsContent,
    staleTime: Infinity, // in-memory; hard reload always refetches (no-store)
  })

  const rawContent = data?.trim() ?? ''
  const content = useMemo(
    () => (rawContent ? rewriteDocImageUrls(rawContent) : ''),
    [rawContent]
  )

  // Assign stable ids to headings, build the hierarchical sidebar TOC (h2
  // sections + their h3 children), and set up a scroll-based scrollspy. Runs
  // after commit, once the async content is in the DOM.
  useEffect(() => {
    const container = contentRef.current
    if (!container || !content) return
    const used = new Map<string, number>()
    const sections: TocNode[] = []
    const idToSection = new Map<string, string>()
    let currentSection: TocNode | null = null

    container.querySelectorAll('h1, h2, h3').forEach((heading) => {
      const text = heading.textContent?.trim() ?? ''
      if (!text || text === '目录') return
      const base = heading.id || text
      const count = used.get(base) ?? 0
      used.set(base, count + 1)
      const id = count === 0 ? base : `${base}-${count}`
      heading.id = id

      const level = Number(heading.tagName[1])
      if (level === 2) {
        currentSection = { id, text, children: [] }
        sections.push(currentSection)
        idToSection.set(id, id)
      } else if (level === 3 && currentSection) {
        currentSection.children.push({ id, text, children: [] })
        idToSection.set(id, currentSection.id)
      }
    })
    setToc(sections)
    idToSectionRef.current = idToSection

    // Deterministic scrollspy: the active heading is the last one whose top is
    // above a reference line near the top of the viewport. Re-queries the DOM
    // fresh on every scroll — the Markdown container re-renders and replaces
    // its nodes, so cached element references would go stale.
    const updateActive = () => {
      const root = contentRef.current
      if (!root) return
      const offset = 150
      let currentId: string | null = null
      for (const heading of root.querySelectorAll('h1, h2, h3')) {
        if (heading.getBoundingClientRect().top <= offset) {
          currentId = (heading as HTMLElement).id || null
        } else {
          break
        }
      }
      if (currentId) {
        setActiveId(currentId)
        setActiveSectionId(idToSectionRef.current.get(currentId) ?? currentId)
      }
    }

    updateActive()
    window.addEventListener('scroll', updateActive, { passive: true })
    window.addEventListener('resize', updateActive)

    // Landing directly on /docs#anchor: scroll now that ids exist.
    const hash = window.location.hash
    if (hash) {
      const target = container.querySelector(`#${CSS.escape(hash.slice(1))}`)
      if (target) {
        target.scrollIntoView({ block: 'start' })
        updateActive()
      }
    }

    return () => {
      window.removeEventListener('scroll', updateActive)
      window.removeEventListener('resize', updateActive)
    }
  }, [content])

  // Precise scroll on TOC click: put the heading just below the fixed header
  // (measured from where the content begins), independent of native anchor
  // navigation / scroll-margin quirks.
  const handleTocClick = (event: MouseEvent<HTMLAnchorElement>, id: string) => {
    event.preventDefault()
    const el = document.querySelector(`#${CSS.escape(id)}`)
    if (!el) return
    const contentTop =
      (layoutRef.current?.getBoundingClientRect().top ?? 0) + window.scrollY
    const elTop = el.getBoundingClientRect().top + window.scrollY
    window.scrollTo({
      top: Math.max(elTop - contentTop - 12, 0),
      behavior: 'smooth',
    })
    history.replaceState(null, '', `#${id}`)
    setActiveId(id)
    setActiveSectionId(idToSectionRef.current.get(id) ?? id)
  }

  const handleContainerClick = (event: MouseEvent<HTMLDivElement>) => {
    const target = event.target
    if (!(target instanceof Element)) return
    const img = target.closest('img')
    if (img) {
      // Open the raw image in a new tab — the source screenshots are small, so
      // an in-page lightbox cannot enlarge them meaningfully.
      window.open(img.currentSrc || img.src, '_blank', 'noopener')
    }
  }

  const renderSidebar = (): ReactNode => (
    <aside className='bg-card/50 border-border/70 top-20 hidden h-fit max-h-[calc(100vh-6rem)] w-60 shrink-0 overflow-y-auto rounded-xl border p-3 pr-2 shadow-sm lg:sticky lg:block'>
      <p className='text-muted-foreground mb-3 px-3 text-xs font-semibold tracking-widest uppercase'>
        {t('Table of Contents')}
      </p>
      <nav className='space-y-0.5 border-l'>
        {toc.map((section) => {
          const sectionActive = activeSectionId === section.id
          return (
            <div key={section.id}>
              <a
                href={`#${section.id}`}
                onClick={(event) => handleTocClick(event, section.id)}
                className={cn(
                  '-ml-px block border-l-2 px-3 py-1 text-sm transition-colors',
                  sectionActive
                    ? 'border-primary text-primary font-medium'
                    : 'border-transparent text-muted-foreground hover:text-foreground'
                )}
              >
                {section.text}
              </a>
              {sectionActive && section.children.length > 0 && (
                <div className='mb-1 ml-2 border-l pl-2'>
                  {section.children.map((child) => (
                    <a
                      key={child.id}
                      href={`#${child.id}`}
                      onClick={(event) => handleTocClick(event, child.id)}
                      className={cn(
                        'block border-l-2 px-2 py-0.5 text-[13px] transition-colors',
                        activeId === child.id
                          ? 'border-primary text-primary font-medium'
                          : 'border-transparent text-muted-foreground/80 hover:text-foreground'
                      )}
                    >
                      {child.text}
                    </a>
                  ))}
                </div>
              )}
            </div>
          )
        })}
      </nav>
    </aside>
  )

  if (isLoading) {
    return (
      <PublicLayout>
        <div className='mx-auto flex max-w-4xl flex-col gap-4 py-12'>
          <Skeleton className='h-8 w-[45%]' />
          <Skeleton className='h-4 w-full' />
          <Skeleton className='h-4 w-[90%]' />
          <Skeleton className='h-4 w-[80%]' />
        </div>
      </PublicLayout>
    )
  }

  if (isError || !content) {
    return (
      <PublicLayout>
        <div className='mx-auto flex max-w-4xl flex-col items-center gap-4 py-24 text-center'>
          <p className='text-muted-foreground'>{t('Failed to load')}</p>
          <Button variant='outline' onClick={() => refetch()}>
            {t('Retry')}
          </Button>
        </div>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout>
      <div
        ref={layoutRef}
        className='mx-auto flex w-full max-w-7xl gap-8 px-4 py-8 sm:px-6 lg:px-8'
      >
        {renderSidebar()}
        <div className='min-w-0 flex-1'>
          <a
            href='/developer-docs'
            className='border-primary/30 bg-primary/5 hover:bg-primary/10 mb-4 flex items-center justify-between gap-4 rounded-xl border px-4 py-3 transition-colors'
          >
            <span className='min-w-0'>
              <span className='block text-sm font-semibold'>
                {t('wan3 Video Integration')}
              </span>
              <span className='text-muted-foreground block truncate text-xs'>
                {t(
                  'Video, image and audio generation · dual-protocol access guide'
                )}
              </span>
            </span>
            <span className='text-primary shrink-0 text-sm'>
              {t('View')} →
            </span>
          </a>
          <div
            ref={contentRef}
            onClick={handleContainerClick}
            className='bg-card border-border/70 rounded-2xl border px-5 py-7 shadow-sm sm:px-8 sm:py-10 lg:px-12 [&_h1]:scroll-mt-20 [&_h2]:scroll-mt-24 [&_h3]:scroll-mt-24 [&_img]:cursor-zoom-in'
          >
            <Markdown>{content}</Markdown>
          </div>
        </div>
      </div>
    </PublicLayout>
  )
}
