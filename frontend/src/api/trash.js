import { fetchURL } from './utils'
import { getApiPath } from '@/utils/url.js'
import { notify } from '@/notify'

// GET /api/trash?source=xxx
export async function listTrash(source) {
  if (!source) {
    throw new Error('source is required')
  }
  try {
    const apiPath = getApiPath('trash', { source })
    const res = await fetchURL(apiPath)
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error fetching trash items')
    throw err
  }
}

// POST /api/trash/restore
export async function restoreFromTrash(source, trashIds) {
  if (!source) {
    throw new Error('source is required')
  }
  if (!Array.isArray(trashIds) || trashIds.length === 0) {
    throw new Error('trashIds must be a non-empty array')
  }
  try {
    const apiPath = getApiPath('trash/restore')
    const res = await fetchURL(apiPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source, trashIds }),
    })
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error restoring items from trash')
    throw err
  }
}

// DELETE /api/trash  (permanently delete selected items)
export async function deleteFromTrash(source, trashIds) {
  if (!source) {
    throw new Error('source is required')
  }
  if (!Array.isArray(trashIds) || trashIds.length === 0) {
    throw new Error('trashIds must be a non-empty array')
  }
  try {
    const apiPath = getApiPath('trash')
    const res = await fetchURL(apiPath, {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source, trashIds }),
    })
    return await res.json()
  } catch (err) {
    notify.showError(err.message || 'Error deleting items from trash')
    throw err
  }
}

// DELETE /api/trash/empty?source=xxx
export async function emptyTrash(source) {
  if (!source) {
    throw new Error('source is required')
  }
  try {
    const apiPath = getApiPath('trash/empty', { source })
    await fetchURL(apiPath, { method: 'DELETE' })
  } catch (err) {
    notify.showError(err.message || 'Error emptying trash')
    throw err
  }
}
