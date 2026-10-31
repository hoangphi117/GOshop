import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')

const targetDirs = [
  path.join(rootDir, '.turbo'),
  path.join(rootDir, 'apps/dashboard/.turbo'),
  path.join(rootDir, 'apps/dashboard/dist'),
  path.join(rootDir, 'apps/storefront/.turbo'),
  path.join(rootDir, 'apps/storefront/.next'),
  path.join(rootDir, 'packages/api-client/.turbo'),
]

let cleanedCount = 0

for (const target of targetDirs) {
  if (fs.existsSync(target)) {
    try {
      fs.rmSync(target, { recursive: true, force: true })
      console.log(`🧹 Deleted: ${path.relative(rootDir, target)}`)
      cleanedCount++
    } catch (err) {
      console.error(`❌ Failed to delete ${target}:`, err.message)
    }
  }
}

if (cleanedCount === 0) {
  console.log('✨ All caches are already clean.')
} else {
  console.log(`✅ Successfully cleaned ${cleanedCount} cache / build folder(s).`)
}
