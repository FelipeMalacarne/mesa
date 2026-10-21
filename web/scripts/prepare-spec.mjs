// Copies ../api/openapi.yaml to .orval/openapi.yaml for orval, quoting flow-mapping
// descriptions that contain a comma (`{ type: string, description: a, b }` is read by
// YAML as an extra key and rejected by orval's validator). The copy is gitignored; the
// contract stays api/openapi.yaml. Once those descriptions are quoted upstream this is a no-op.
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const src = resolve(here, '../../api/openapi.yaml')
const out = resolve(here, '../.orval/openapi.yaml')

const flowDescription = /^(\s*.*\{.*\bdescription: )([^"'\n][^\n]*?)(\s*\}\s*)$/
let fixed = 0
const lines = readFileSync(src, 'utf8').split('\n').map((line) => {
  // `enum: [null, bool, …]` reads the unquoted null as a YAML null, which drops the "null" kind
  // from Value.k; quote it so the generated client matches the wire form.
  if (/^\s*enum: \[null,/.test(line)) {
    fixed++
    return line.replace('[null,', '["null",')
  }
  const m = flowDescription.exec(line)
  if (!m || !m[2].includes(',')) return line
  fixed++
  return `${m[1]}"${m[2].replace(/"/g, '\\"')}"${m[3]}`
})

mkdirSync(dirname(out), { recursive: true })
writeFileSync(out, lines.join('\n'))
console.log(`prepared ${out} (${fixed} fix${fixed === 1 ? '' : 'es'} applied)`)
