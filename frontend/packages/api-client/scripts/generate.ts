import { execSync } from 'node:child_process'
import { existsSync, mkdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

// Candidates for openapi / swagger spec file
const specCandidates = [
  path.resolve(__dirname, '../../../../backend/docs/openapi.json'),
  path.resolve(__dirname, '../../../../backend/docs/swagger.json'),
]

const specPath = specCandidates.find((candidate) => existsSync(candidate))

if (!specPath) {
  console.error('❌ Could not locate backend/docs/openapi.json or swagger.json')
  process.exit(1)
}

const outputDir = path.resolve(__dirname, '../src/generated')
if (!existsSync(outputDir)) {
  mkdirSync(outputDir, { recursive: true })
}

const outputPath = path.resolve(outputDir, 'schema.d.ts')

console.log(`🔄 Generating TypeScript types from ${specPath} ...`)

try {
  execSync(`npx openapi-typescript "${specPath}" -o "${outputPath}"`, {
    stdio: 'inherit',
    cwd: path.resolve(__dirname, '..'),
  })
  console.log(`✅ Schema types generated at: ${outputPath}`)
} catch (error) {
  console.error('❌ Failed to generate schema types:', error)
  process.exit(1)
}
