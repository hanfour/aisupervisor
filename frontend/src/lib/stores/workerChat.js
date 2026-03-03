import { writable, get } from 'svelte/store'

// Shared state for the worker chat drawer
export const chatOpen = writable(false)
export const chatWorkerID = writable('')
export const chatWorkerName = writable('')
export const chatWorkerAvatar = writable('')
export const chatMessages = writable([]) // { role: 'user'|'assistant', content: string }
export const chatLoading = writable(false)

export function openChat(workerID, workerName, workerAvatar) {
  chatWorkerID.set(workerID)
  chatWorkerName.set(workerName)
  chatWorkerAvatar.set(workerAvatar)
  chatMessages.set([])
  chatOpen.set(true)
}

export function closeChat() {
  chatOpen.set(false)
}

export async function sendMessage(text) {
  if (!text.trim()) return
  if (!window.go?.gui?.CompanyApp) return

  const workerID = get(chatWorkerID)

  chatMessages.update(msgs => [...msgs, { role: 'user', content: text }])
  chatLoading.set(true)

  try {
    const messages = get(chatMessages)
    const resp = await window.go.gui.CompanyApp.ChatWithWorker(workerID, messages)
    if (resp && resp.content) {
      chatMessages.update(msgs => [...msgs, { role: 'assistant', content: resp.content }])
    }
  } catch (e) {
    const errMsg = e?.message || (typeof e === 'string' ? e : JSON.stringify(e)) || 'Unknown error'
    chatMessages.update(msgs => [...msgs, { role: 'assistant', content: `[Error: ${errMsg}]` }])
  } finally {
    chatLoading.set(false)
  }
}
