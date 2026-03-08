import { LogDebug, LogError } from '../../wailsjs/runtime/runtime'

export interface DebugEntry {
  timestamp: string
  level: 'info' | 'error'
  source: string
  message: string
  detail?: string
}

const DEBUG_EVENT_NAME = 'cpa:debug'
const DEBUG_BUFFER_LIMIT = 200
let debugEnabled = false
const debugBuffer: DebugEntry[] = []

function stringifyDetail(detail: unknown): string | undefined {
  if (detail === undefined || detail === null) {
    return undefined
  }
  if (typeof detail === 'string') {
    return detail
  }
  try {
    return JSON.stringify(detail, null, 2)
  } catch {
    return String(detail)
  }
}

function push(level: DebugEntry['level'], source: string, message: string, detail?: unknown) {
  const entry: DebugEntry = {
    timestamp: new Date().toISOString(),
    level,
    source,
    message,
    detail: stringifyDetail(detail),
  }

  debugBuffer.push(entry)
  if (debugBuffer.length > DEBUG_BUFFER_LIMIT) {
    debugBuffer.splice(0, debugBuffer.length - DEBUG_BUFFER_LIMIT)
  }

  if (!debugEnabled) {
    return
  }

  window.dispatchEvent(new CustomEvent<DebugEntry>(DEBUG_EVENT_NAME, { detail: entry }))

  const line = `[${entry.timestamp}] [${source}] ${message}${entry.detail ? ` :: ${entry.detail}` : ''}`
  try {
    if (level === 'error') {
      LogError(line)
    } else {
      LogDebug(line)
    }
  } catch {
    console[level === 'error' ? 'error' : 'debug'](line)
  }
}

export function emitDebug(source: string, message: string, detail?: unknown) {
  push('info', source, message, detail)
}

export function emitDebugError(source: string, message: string, error?: unknown) {
  push('error', source, message, error instanceof Error ? `${error.name}: ${error.message}` : error)
}

export function debugEventName() {
  return DEBUG_EVENT_NAME
}

export function setDebugEnabled(enabled: boolean) {
  debugEnabled = enabled
}

export function isDebugEnabled() {
  return debugEnabled
}

export function snapshotDebugEntries() {
  return [...debugBuffer]
}
