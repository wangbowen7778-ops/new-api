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
import { MAX_HISTORY_ITEMS, STORAGE_KEYS } from '../constants'
import type { DrawingConfig, DrawingRecord } from '../types'

export function loadConfig(): Partial<DrawingConfig> {
  try {
    const saved = localStorage.getItem(STORAGE_KEYS.CONFIG)
    if (!saved) return {}

    const parsed = JSON.parse(saved) as Partial<DrawingConfig>
    if (!parsed || typeof parsed === 'object') return parsed
    return {}
  } catch {
    return {}
  }
}

export function saveConfig(config: DrawingConfig): void {
  try {
    localStorage.setItem(STORAGE_KEYS.CONFIG, JSON.stringify(config))
  } catch {
    // ignore quota errors - config is a convenience, not critical state
  }
}

/**
 * History lives in IndexedDB: generated images are stored as multi-megabyte
 * base64 data URLs, far beyond the ~5MB localStorage quota that used to drop
 * every record but the newest one. Config stays in localStorage because it is
 * a tiny JSON object.
 */
const HISTORY_DB_NAME = 'drawing-history'
const HISTORY_DB_VERSION = 1
const HISTORY_STORE = 'records'

function openHistoryDb(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(HISTORY_DB_NAME, HISTORY_DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(HISTORY_STORE)) {
        db.createObjectStore(HISTORY_STORE, { keyPath: 'id' })
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () =>
      reject(request.error ?? new Error('failed to open drawing history db'))
  })
}

function readLegacyHistory(): DrawingRecord[] {
  try {
    const saved = localStorage.getItem(STORAGE_KEYS.HISTORY)
    if (!saved) return []
    const parsed = JSON.parse(saved) as DrawingRecord[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function sortNewestFirst(records: DrawingRecord[]): DrawingRecord[] {
  return records
    .slice(0, MAX_HISTORY_ITEMS)
    .sort((a, b) => (b.createdAt ?? 0) - (a.createdAt ?? 0))
}

async function writeAllRecords(records: DrawingRecord[]): Promise<void> {
  const db = await openHistoryDb()
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(HISTORY_STORE, 'readwrite')
      const store = tx.objectStore(HISTORY_STORE)
      for (const record of records) {
        store.put(record)
      }
      tx.oncomplete = () => resolve()
      tx.onerror = () =>
        reject(tx.error ?? new Error('failed to write drawing history'))
    })
  } finally {
    db.close()
  }
}

export async function loadHistory(): Promise<DrawingRecord[]> {
  try {
    const db = await openHistoryDb()
    let stored: DrawingRecord[]
    try {
      stored = await new Promise<DrawingRecord[]>((resolve, reject) => {
        const tx = db.transaction(HISTORY_STORE, 'readonly')
        const request = tx.objectStore(HISTORY_STORE).getAll()
        request.onsuccess = () =>
          resolve((request.result as DrawingRecord[]) ?? [])
        request.onerror = () =>
          reject(request.error ?? new Error('failed to read drawing history'))
      })
    } finally {
      db.close()
    }

    // One-time migration from the old localStorage-backed history.
    const legacy = readLegacyHistory()
    if (legacy.length > 0) {
      const legacyIds = new Set(legacy.map((record) => record.id))
      const merged = [
        ...stored.filter((record) => !legacyIds.has(record.id)),
        ...legacy,
      ]
      const fitted = sortNewestFirst(merged)
      await writeAllRecords(fitted)
      localStorage.removeItem(STORAGE_KEYS.HISTORY)
      return fitted
    }

    return sortNewestFirst(stored)
  } catch {
    // IndexedDB unavailable (private mode, blocked storage) - the in-memory
    // state in the page still works for the current session.
    return sortNewestFirst(readLegacyHistory())
  }
}

// Records are immutable once created (only prepended or deleted), so the
// persist pass only writes ids it has not seen and removes ones that vanished.
let knownRecordIds: Set<string> | null = null

export function saveHistory(records: DrawingRecord[]): void {
  const current = sortNewestFirst(records)
  const nextIds = new Set(current.map((record) => record.id))

  void (async () => {
    const db = await openHistoryDb()
    try {
      await new Promise<void>((resolve, reject) => {
        const tx = db.transaction(HISTORY_STORE, 'readwrite')
        const store = tx.objectStore(HISTORY_STORE)
        for (const record of current) {
          if (!knownRecordIds || !knownRecordIds.has(record.id)) {
            store.put(record)
          }
        }
        if (knownRecordIds) {
          for (const id of knownRecordIds) {
            if (!nextIds.has(id)) {
              store.delete(id)
            }
          }
        }
        tx.oncomplete = () => resolve()
        tx.onerror = () =>
          reject(tx.error ?? new Error('failed to persist drawing history'))
      })
      knownRecordIds = nextIds
    } catch {
      // give up silently; in-memory history still works this session
    } finally {
      db.close()
    }
  })()
}

export function clearHistoryStorage(): void {
  knownRecordIds = new Set()
  try {
    localStorage.removeItem(STORAGE_KEYS.HISTORY)
  } catch {
    // nothing meaningful to do
  }
  void (async () => {
    try {
      const db = await openHistoryDb()
      try {
        await new Promise<void>((resolve, reject) => {
          const tx = db.transaction(HISTORY_STORE, 'readwrite')
          tx.objectStore(HISTORY_STORE).clear()
          tx.oncomplete = () => resolve()
          tx.onerror = () => reject(tx.error ?? new Error('failed to clear'))
        })
      } finally {
        db.close()
      }
    } catch {
      // nothing meaningful to do
    }
  })()
}
