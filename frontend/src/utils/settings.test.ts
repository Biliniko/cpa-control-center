import { describe, expect, it } from 'vitest'
import { createDefaultSettings, validateSettings } from '@/utils/settings'

describe('validateSettings', () => {
  it('accepts a valid CPA profile', () => {
    const settings = createDefaultSettings()
    expect(settings.detailedLogs).toBe(false)
    expect(settings.scanStrategy).toBe('full')
    expect(settings.scanBatchSize).toBe(1000)
    expect(settings.skipKnown401).toBe(true)
    settings.baseUrl = 'https://example.com'
    settings.managementToken = 'token'

    expect(validateSettings(settings)).toEqual({})
  })

  it('rejects missing or malformed core fields', () => {
    const settings = createDefaultSettings()
    settings.baseUrl = 'example.com'
    settings.managementToken = ''
    settings.probeWorkers = 0

    expect(validateSettings(settings)).toMatchObject({
      baseUrl: expect.any(String),
      managementToken: expect.any(String),
      probeWorkers: expect.any(String),
    })
  })

  it('rejects invalid scheduler settings when enabled', () => {
    const settings = createDefaultSettings()
    settings.baseUrl = 'https://example.com'
    settings.managementToken = 'token'
    settings.schedule.enabled = true
    settings.schedule.mode = 'scan'
    settings.schedule.cron = 'not-a-cron'

    expect(validateSettings(settings)).toMatchObject({
      scheduleCron: expect.any(String),
    })
  })

  it('accepts a valid 5-field cron expression', () => {
    const settings = createDefaultSettings()
    settings.baseUrl = 'https://example.com'
    settings.managementToken = 'token'
    settings.schedule.enabled = true
    settings.schedule.mode = 'maintain'
    settings.schedule.cron = '0 */6 * * *'

    expect(validateSettings(settings)).toEqual({})
  })

  it('rejects invalid incremental scan settings', () => {
    const settings = createDefaultSettings()
    settings.baseUrl = 'https://example.com'
    settings.managementToken = 'token'
    settings.scanStrategy = 'incremental'
    settings.scanBatchSize = 0

    expect(validateSettings(settings)).toMatchObject({
      scanBatchSize: expect.any(String),
    })
  })
})
