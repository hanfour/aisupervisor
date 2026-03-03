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
