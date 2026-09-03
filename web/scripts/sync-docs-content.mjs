import {
  copyFile,
  mkdir,
  readdir,
  readFile,
  rm,
  stat,
  writeFile,
} from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const scriptDir = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(scriptDir, '..')
const repoDir = path.resolve(webDir, '..')
const sourcePath = path.join(repoDir, 'ShwllAPI使用文档.md')
const outputDir = path.join(webDir, 'public', 'docs-content')
const outputPath = path.join(outputDir, 'ShwllAPI使用文档.md')
const copiedImages = new Map()

function isExternalUrl(url) {
  return /^(?:[a-z][a-z0-9+.-]*:|\/|#)/i.test(url)
}

function resolveSourceImage(url) {
  const decodedUrl = decodeURIComponent(url).replaceAll('\\', '/')
  const sourceImage = path.resolve(repoDir, decodedUrl)
  const relativePath = path.relative(repoDir, sourceImage)

  if (
    relativePath.startsWith(`..${path.sep}`) ||
    relativePath === '..' ||
    path.isAbsolute(relativePath)
  ) {
    throw new Error(`Documentation image is outside the repository: ${url}`)
  }

  return sourceImage
}

await mkdir(outputDir, { recursive: true })
const source = await readFile(sourcePath, 'utf8')
const imagePattern = /!\[([^\]]*)\]\(([^)]+)\)/g
let match
let cursor = 0
let output = ''

while ((match = imagePattern.exec(source)) !== null) {
  output += source.slice(cursor, match.index)
  cursor = match.index + match[0].length

  const target = match[2].trim()
  const url = target.split(/\s+/)[0] ?? ''
  if (!url) {
    output += match[0]
    continue
  }
  if (/^[A-Za-z]:[\\/]/.test(url)) {
    throw new Error(`Documentation image uses a local absolute path: ${url}`)
  }
  if (isExternalUrl(url)) {
    output += match[0]
    continue
  }

  const sourceImage = resolveSourceImage(url)
  const imageName = path.basename(sourceImage)
  const previousSource = copiedImages.get(imageName)
  if (previousSource && previousSource !== sourceImage) {
    throw new Error(
      `Documentation images share the output name ${imageName}: ${previousSource}, ${sourceImage}`
    )
  }

  const imageStats = await stat(sourceImage).catch(() => null)
  if (!imageStats?.isFile()) {
    throw new Error(`Documentation image does not exist: ${url}`)
  }

  copiedImages.set(imageName, sourceImage)
  const suffix = target.slice(url.length)
  output += `![${match[1]}](${imageName}${suffix})`
}

output += source.slice(cursor)

if (/Typora[\\/]/i.test(output)) {
  throw new Error('Generated documentation contains a Typora image path')
}

for (const entry of await readdir(outputDir, { withFileTypes: true })) {
  if (
    entry.isFile() &&
    /\.(?:png|jpe?g|gif|webp|svg)$/i.test(entry.name) &&
    !copiedImages.has(entry.name)
  ) {
    await rm(path.join(outputDir, entry.name))
  }
}

for (const [imageName, sourceImage] of copiedImages) {
  await copyFile(sourceImage, path.join(outputDir, imageName))
}
await writeFile(outputPath, output, 'utf8')

console.log(
  `Synced ShwllAPI使用文档.md and ${copiedImages.size} images to web/public/docs-content`
)
