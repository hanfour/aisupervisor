import { writable } from 'svelte/store'

export const characterProfiles = writable({})
export const relationships = writable({})

export async function loadCharacterProfile(workerId) {
    if (!window.go?.gui?.CompanyApp) return null
    try {
        const profile = await window.go.gui.CompanyApp.GetCharacterProfile(workerId)
        if (profile) {
            characterProfiles.update(p => ({ ...p, [workerId]: profile }))
        }
        return profile
    } catch (e) {
        console.error('Failed to load character profile:', e)
        return null
    }
}

export async function loadWorkerRelationships(workerId) {
    if (!window.go?.gui?.CompanyApp) return []
    try {
        const rels = await window.go.gui.CompanyApp.GetWorkerRelationships(workerId)
        relationships.update(r => ({ ...r, [workerId]: rels || [] }))
        return rels || []
    } catch (e) {
        console.error('Failed to load relationships:', e)
        return []
    }
}

export async function loadAllRelationships(workerIds) {
    if (!window.go?.gui?.CompanyApp) return []
    const allRels = []
    const seen = new Set()
    for (const id of workerIds) {
        try {
            const rels = await window.go.gui.CompanyApp.GetWorkerRelationships(id)
            if (rels) {
                for (const r of rels) {
                    const key = r.workerA < r.workerB ? `${r.workerA}-${r.workerB}` : `${r.workerB}-${r.workerA}`
                    if (!seen.has(key)) {
                        seen.add(key)
                        allRels.push(r)
                    }
                }
                relationships.update(rv => ({ ...rv, [id]: rels }))
            }
        } catch (e) { /* ignore */ }
    }
    return allRels
}

export function initPersonalityEvents() {
    if (!window.runtime) return

    window.runtime.EventsOn('personality:mood', (data) => {
        if (data?.workerId) {
            loadCharacterProfile(data.workerId)
        }
    })
    window.runtime.EventsOn('personality:relationship', (data) => {
        if (data?.workerId) {
            loadWorkerRelationships(data.workerId)
        }
    })
    window.runtime.EventsOn('personality:narrative', (data) => {
        if (data?.workerId) {
            loadCharacterProfile(data.workerId)
        }
    })
}

export async function generateNarrative(workerId) {
    if (!window.go?.gui?.CompanyApp) return
    try {
        await window.go.gui.CompanyApp.GenerateNarrative(workerId)
        await loadCharacterProfile(workerId)
    } catch (e) {
        console.error('Failed to generate narrative:', e)
        throw e
    }
}
