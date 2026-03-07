// Social graph — analyzes relationships to detect cliques and buddy pairs

export class SocialGraph {
  constructor() {
    this._relationships = new Map() // "idA-idB" → RelationshipDTO
    this._cliques = []              // Array of Set<workerId>
    this._dirty = true
  }

  setRelationships(relationships) {
    this._relationships.clear()
    for (const r of relationships) {
      const key = this._key(r.workerA, r.workerB)
      this._relationships.set(key, r)
    }
    this._dirty = true
  }

  getAffinity(idA, idB) {
    const r = this._relationships.get(this._key(idA, idB))
    return r?.affinity ?? 50
  }

  getRelationship(idA, idB) {
    return this._relationships.get(this._key(idA, idB)) ?? null
  }

  getCliqueFor(workerId) {
    this._ensureCliques()
    for (const clique of this._cliques) {
      if (clique.has(workerId)) return [...clique]
    }
    return [workerId]
  }

  getAllCliques() {
    this._ensureCliques()
    return this._cliques.map(c => [...c])
  }

  // Find best buddy (highest affinity) for a worker
  getBestBuddy(workerId, workerIds) {
    let best = null
    let bestAffinity = 0
    for (const otherId of workerIds) {
      if (otherId === workerId) continue
      const aff = this.getAffinity(workerId, otherId)
      if (aff > bestAffinity) {
        bestAffinity = aff
        best = otherId
      }
    }
    return bestAffinity > 60 ? best : null
  }

  // ── Private ─────────────────────────────────────────────────────────────

  _key(idA, idB) {
    return idA < idB ? `${idA}-${idB}` : `${idB}-${idA}`
  }

  _ensureCliques() {
    if (!this._dirty) return
    this._dirty = false
    this._detectCliques()
  }

  _detectCliques() {
    // Simple greedy clique detection:
    // Workers with mutual affinity > 70 and interaction > 10 are in the same clique
    const edges = new Map() // workerId → Set<workerId>

    for (const [, r] of this._relationships) {
      if (r.affinity > 70 && r.interactionCount > 10) {
        if (!edges.has(r.workerA)) edges.set(r.workerA, new Set())
        if (!edges.has(r.workerB)) edges.set(r.workerB, new Set())
        edges.get(r.workerA).add(r.workerB)
        edges.get(r.workerB).add(r.workerA)
      }
    }

    // BFS to find connected components
    const visited = new Set()
    this._cliques = []

    for (const [workerId] of edges) {
      if (visited.has(workerId)) continue
      const clique = new Set()
      const queue = [workerId]
      while (queue.length > 0) {
        const current = queue.shift()
        if (visited.has(current)) continue
        visited.add(current)
        clique.add(current)
        const neighbors = edges.get(current)
        if (neighbors) {
          for (const n of neighbors) {
            if (!visited.has(n)) queue.push(n)
          }
        }
      }
      if (clique.size >= 2) {
        this._cliques.push(clique)
      }
    }
  }
}
