#!/usr/bin/env node

import fs from 'node:fs'
import path from 'node:path'

const root = path.resolve(process.argv[2] || 'docs-site')
const configPath = path.join(root, '.vitepress', 'config.mts')

function markdownFiles(directory) {
  const files = []
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.name === '.vitepress' || entry.name === 'public') continue
    const full = path.join(directory, entry.name)
    if (entry.isDirectory()) files.push(...markdownFiles(full))
    else if (entry.isFile() && entry.name.endsWith('.md')) files.push(full)
  }
  return files
}

function routeFor(file) {
  const relative = path.relative(root, file).split(path.sep).join('/')
  if (relative === 'index.md') return '/'
  if (relative.endsWith('/index.md')) return `/${relative.slice(0, -'/index.md'.length)}`
  return `/${relative.slice(0, -'.md'.length)}`
}

function normalizeRoute(value, source) {
  const withoutHash = value.split('#', 1)[0].split('?', 1)[0]
  if (!withoutHash) return routeFor(source)
  const route = withoutHash.startsWith('/')
    ? withoutHash
    : `/${path.posix.normalize(path.posix.join(path.posix.dirname(path.relative(root, source).split(path.sep).join('/')), withoutHash))}`
  const normalized = route.replace(/\.md$/, '').replace(/\.html$/, '').replace(/\/+/g, '/')
  if (normalized === '/') return '/'
  return normalized.replace(/\/$/, '')
}

function anchorFor(value) {
  const hash = value.split('#')[1]
  return hash ? decodeURIComponent(hash).toLowerCase() : ''
}

function countOccurrences(text, needle) {
  return text.split(needle).length - 1
}

function headingAnchor(text) {
  return text
    .trim()
    .toLowerCase()
    .replace(/<[^>]+>/g, '')
    .replace(/[^\p{L}\p{N} _-]/gu, '')
    .replace(/\s+/g, '-')
}

const files = markdownFiles(root)
const routes = new Map(files.map((file) => [routeFor(file), file]))
const anchors = new Map()
for (const file of files) {
  const found = new Set()
  for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const heading = line.match(/^#{1,6}\s+(.+?)\s*#*$/)
    if (heading) found.add(headingAnchor(heading[1]))
    for (const explicit of line.matchAll(/(?:id|name)=["']([^"']+)["']/g)) found.add(explicit[1].toLowerCase())
  }
  anchors.set(routeFor(file), found)
}

let linkCount = 0
let errorCount = 0
for (const file of files) {
  const text = fs.readFileSync(file, 'utf8')
  for (const match of text.matchAll(/\]\(([^)\s]+)(?:\s+[^)]*)?\)/g)) {
    const target = match[1]
    if (/^(?:https?:|mailto:|tel:|\/\/)/i.test(target)) continue
    linkCount += 1
    const route = normalizeRoute(target, file)
    if (!routes.has(route)) {
      console.error(`docs links: ${path.relative(root, file)} -> missing ${target} (${route})`)
      errorCount += 1
      continue
    }
    const anchor = anchorFor(target)
    if (anchor && !anchors.get(route).has(anchor)) {
      console.error(`docs links: ${path.relative(root, file)} -> missing anchor ${target}`)
      errorCount += 1
    }
  }
}

if (!fs.existsSync(configPath)) {
  console.error(`docs links: navigation config not found: ${configPath}`)
  errorCount += 1
} else {
  const config = fs.readFileSync(configPath, 'utf8')
  const navigationLinks = [...config.matchAll(/link:\s*['"]([^'"]+)['"]/g)]
    .map((match) => match[1])
    .filter((link) => link.startsWith('/'))
  for (const link of navigationLinks) {
    if (!routes.has(link.replace(/\/$/, ''))) {
      console.error(`docs links: navigation points to missing page ${link}`)
      errorCount += 1
    }
  }
  for (const group of ['Start', 'Daily Delivery', 'Agents', 'Advanced Orchestration', 'Reference']) {
    const marker = `text: '${group}'`
    if (countOccurrences(config, marker) < 2) {
      console.error(`docs links: required group ${group} must appear in both top navigation and sidebar`)
      errorCount += 1
    }
  }
  for (const entry of ['/install', '/quickstart', '/ticket-workflow', '/branch-policy', '/troubleshooting', '/agent-operator-skill', '/command-reference']) {
    if (!navigationLinks.includes(entry)) {
      console.error(`docs links: required entry page is not discoverable from navigation: ${entry}`)
      errorCount += 1
    }
  }
}

if (errorCount) process.exitCode = 1
else console.log(`docs links: checked ${linkCount} internal links across ${files.length} pages and navigation contract`)
