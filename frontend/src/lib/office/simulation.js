// Simulation engine — orchestrates character activities in the pixel office
// Drives the activity state machine for each worker and reacts to backend events

import { getZoneTiles } from './layout.js'

// ── Message pools ─────────────────────────────────────────────────────────────

const DISCUSSION_MESSAGES = [
  'Discussing API design',
  'Code review feedback',
  'Sprint planning',
  'Bug triage',
  'Architecture discussion',
  'Pair programming',
  'PR review',
  'Design patterns',
  'Performance tuning',
  'Refactoring strategy',
  'Test coverage',
  'Deployment checklist',
  'Dependency update',
  'Security review',
  'Database schema',
  'Integration testing',
  'Onboarding plan',
  'OKR alignment',
  'Technical debt',
  'Release planning',
]

const THOUGHT_MESSAGES = [
  'Hmm, interesting approach...',
  'This could work...',
  'Need more coffee...',
  'Almost got it!',
  'Why is this broken?',
  'Off-by-one error?',
  'Need to refactor this...',
  'Docs are unclear...',
  'Stack overflow to the rescue',
  'Rubber duck time',
  'Just one more fix...',
  'Should write tests first',
  'Cache invalidation...',
  'Edge case found!',
  'Time to ask for help',
  'Log all the things',
  'This is elegant',
  'Technical debt grows...',
  'Naming things is hard',
  'Works on my machine',
]

const WORKING_THOUGHTS = [
  'Debugging...',
  'Refactoring...',
  'Almost there...',
  'Writing tests...',
  'Optimising...',
  'Fixing lint...',
  'Reading docs...',
  'Adding feature...',
  'Checking logs...',
  'Pushing to git...',
]

const MEETING_TOPICS = [
  'Sprint Review',
  'Feature Planning',
  'Tech Debt Discussion',
  'Team Standup',
  'Architecture Review',
  'Retrospective',
  'Design Sync',
  'Incident Post-mortem',
  'Roadmap Planning',
  'Hiring Discussion',
]

const TIRED_MESSAGES = [
    '好累...', '需要咖啡☕', '休息一下...',
    '快下班了嗎...', '打個哈欠~'
]
const COMFORTING_MESSAGES = [
    '沒關係的', '要不要去喝杯咖啡？',
    '下次會更好', '一起加油！'
]
const CELEBRATION_MESSAGES = [
    '太棒了！🎉', '完成了！', '慶祝！',
    '辛苦了！', 'GG！'
]
const PAIR_PROG_MESSAGES = [
    '這邊看看...', '一起 debug', '你覺得呢？',
    '試試這個方法', '我來寫你來看'
]

// ── Helpers ──────────────────────────────────────────────────────────────────

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)]
}

function randBetween(min, max) {
  return min + Math.random() * (max - min)
}

// Pick a random walkable tile from a named zone
function randomZoneTile(zoneName) {
  const tiles = getZoneTiles(zoneName)
  if (!tiles.length) return null
  return tiles[Math.floor(Math.random() * tiles.length)]
}

// ── SimulationEngine ─────────────────────────────────────────────────────────

export class SimulationEngine {
  constructor(renderer) {
    this.renderer = renderer
    this.workerStates = new Map()   // workerId → { state, data, timer }
    this.speed = 1.0
    this.paused = false
    this.tickInterval = 5000        // ms between auto-ticks (unadjusted)
    this.lastTick = 0
    this.activityLog = []
  }

  // ── Public API ─────────────────────────────────────────────────────────────

  setWorkers(workers) {
    const ids = new Set(workers.map(w => w.id))

    // Remove stale entries
    for (const id of this.workerStates.keys()) {
      if (!ids.has(id)) this.workerStates.delete(id)
    }

    // Add new workers defaulting to 'at-desk'
    for (const w of workers) {
      if (!this.workerStates.has(w.id)) {
        this.workerStates.set(w.id, { state: 'at-desk', data: {}, timer: 0 })
      }
    }

    this._workers = workers
  }

  setProfiles(profileMap) {
    this.profiles = profileMap // Map<workerId, CharacterProfileDTO>
  }

  update(deltaMs) {
    if (this.paused || !this._workers?.length) return
    const adjusted = deltaMs * this.speed

    this._updateActivities(adjusted)

    this.lastTick += adjusted
    if (this.lastTick >= this.tickInterval) {
      this.lastTick = 0
      this._autoTick()
    }
  }

  handleEvent(event) {
    if (!event?.type) return
    switch (event.type) {
      case 'task_assigned':
        this._onTaskAssigned(event)
        break
      case 'task_completed':
        this._onTaskCompleted(event)
        break
      case 'review_started':
        this._onReviewStarted(event)
        break
      case 'review_completed':
        this._onReviewCompleted(event)
        break
    }
  }

  setSpeed(multiplier) {
    this.speed = Math.max(0.1, multiplier)
  }

  setPaused(paused) {
    this.paused = paused
  }

  getActivityLog() {
    return this.activityLog.slice(-20)
  }

  // ── Activity updates ──────────────────────────────────────────────────────

  _updateActivities(adjustedDelta) {
    for (const [id, ws] of this.workerStates) {
      switch (ws.state) {
        case 'walking-to-zone':
        case 'walking-to-person':
          if (!this.renderer.isWorkerMoving(id)) {
            this._onArrived(id, ws)
          }
          break

        case 'discussing':
        case 'in-meeting':
        case 'at-watercooler':
        case 'thinking': {
          ws.timer -= adjustedDelta
          if (ws.timer <= 0) {
            this._startReturning(id, ws)
          }
          break
        }

        case 'returning':
          if (!this.renderer.isWorkerMoving(id)) {
            ws.state = 'at-desk'
            ws.data = {}
            ws.timer = 0
          }
          break
      }
    }
  }

  _onArrived(id, ws) {
    const { activity } = ws.data
    switch (activity) {
      case 'watercooler': {
        const dur = randBetween(3000, 5000)
        ws.state = 'at-watercooler'
        ws.timer = dur
        this.renderer.showSpeech(id, pick(THOUGHT_MESSAGES), dur)
        this._log(id, 'at watercooler')
        break
      }
      case 'discussion': {
        const { partnerId, topic } = ws.data
        const dur = randBetween(4000, 6000)
        ws.state = 'discussing'
        ws.timer = dur
        const bubbleId = this.renderer.showDiscussion(id, partnerId, topic, dur)
        ws.data.bubbleId = bubbleId
        this._log(id, `discussing: ${topic}`)
        break
      }
      case 'meeting': {
        // Only the first walker arriving triggers the meeting bubble
        const { meetingIds, topic } = ws.data
        const allArrived = meetingIds.every(mid => {
          const s = this.workerStates.get(mid)
          return s && (s.state === 'in-meeting' || (s.state === 'walking-to-zone' && !this.renderer.isWorkerMoving(mid)))
        })
        const dur = randBetween(8000, 12000)
        ws.state = 'in-meeting'
        ws.timer = dur

        // Only first participant to arrive shows the bubble
        if (!ws.data.bubbleShown) {
          ws.data.bubbleShown = true
          const bubbleId = this.renderer.showMeeting(meetingIds, topic, dur)
          // Share bubble ID with all meeting participants so returning clears them
          for (const mid of meetingIds) {
            const ms = this.workerStates.get(mid)
            if (ms) {
              ms.data.sharedBubbleId = bubbleId
              if (ms.state === 'walking-to-zone') {
                ms.state = 'in-meeting'
                ms.timer = dur
              }
            }
          }
        }
        this._log(id, `in meeting: ${topic}`)
        break
      }
      case 'patrol-visit': {
        const dur = 2000
        ws.state = 'at-watercooler'  // reuse timed-wait state
        ws.timer = dur
        this.renderer.showSpeech(id, 'Checking in...', dur)
        break
      }
      case 'manager-meeting': {
        const dur = randBetween(6000, 10000)
        ws.state = 'in-meeting'
        ws.timer = dur
        this.renderer.showThought(id, pick(MEETING_TOPICS), dur)
        this._log(id, 'manager meeting')
        break
      }
    }
  }

  _startReturning(id, ws) {
    if (ws.data.bubbleId) {
      this.renderer.clearBubble(ws.data.bubbleId)
    }
    if (ws.data.sharedBubbleId) {
      this.renderer.clearBubble(ws.data.sharedBubbleId)
    }
    this.renderer.clearWorkerBubbles(id)
    this.renderer.returnWorkerToDesk(id)
    ws.state = 'returning'
    ws.data = {}
    ws.timer = 0
  }

  // ── Auto-tick ─────────────────────────────────────────────────────────────

  _autoTick() {
    if (!this._workers?.length) return

    const idleWorkers    = this._workersWithState('at-desk', 'idle')
    const workingWorkers = this._workersWithState('at-desk', 'working')
    const managers       = this._workersWithState('at-desk', null, 'manager')
    const allAtDesk      = this._workersWithState('at-desk')

    // Random meetings: 5% when 3+ workers idle
    if (idleWorkers.length >= 3 && Math.random() < 0.05) {
      const count = Math.min(idleWorkers.length, Math.floor(randBetween(2, 5)))
      const picked = this._sample(idleWorkers, count)
      if (picked.length >= 2) {
        this._startMeeting(picked, pick(MEETING_TOPICS))
        return
      }
    }

    // Manager patrol / meeting room
    for (const w of managers) {
      const ws = this.workerStates.get(w.id)
      if (ws.state !== 'at-desk') continue
      const r = Math.random()
      if (r < 0.15) {
        this._startPatrol(w.id, allAtDesk.filter(x => x.id !== w.id))
        break
      } else if (r < 0.25) {
        this._startManagerMeeting(w.id)
        break
      }
    }

    // Idle workers
    for (const w of idleWorkers) {
      const ws = this.workerStates.get(w.id)
      if (ws.state !== 'at-desk') continue
      const r = Math.random()
      if (r < 0.15) {
        this._startWatercooler(w.id)
      } else if (r < 0.25) {
        const partner = this._randomOther(idleWorkers, w.id)
        if (partner) this._startDiscussion(w.id, partner.id)
      } else if (r < 0.30) {
        this._startThinking(w.id)
      }
    }

    // Working workers
    for (const w of workingWorkers) {
      const ws = this.workerStates.get(w.id)
      if (ws.state !== 'at-desk') continue
      const r = Math.random()
      if (r < 0.20) {
        const dur = 3000
        this.renderer.showThought(w.id, pick(WORKING_THOUGHTS), dur)
        this._log(w.id, 'thinking')
      } else if (r < 0.25) {
        const partner = this._randomOther(workingWorkers, w.id)
        if (partner) this._startQuickQuestion(w.id, partner.id)
      }
    }

    // Pair Programming: 8% chance for two idle engineers with high affinity
    if (this.profiles && idleWorkers.length >= 2) {
      for (let i = 0; i < idleWorkers.length && Math.random() < 0.08; i++) {
        const w1 = idleWorkers[i]
        const p1 = this.profiles.get(w1.id)
        if (!p1) continue
        for (let j = i + 1; j < idleWorkers.length; j++) {
          const w2 = idleWorkers[j]
          // Check state is at-desk for both
          const s1 = this.workerStates.get(w1.id)
          const s2 = this.workerStates.get(w2.id)
          if (s1?.state === 'at-desk' && s2?.state === 'at-desk') {
            this._startPairProgramming(w1, w2)
            break
          }
        }
      }
    }

    // Comforting: check for stressed workers
    if (this.profiles) {
      for (const w of this._workers || []) {
        const profile = this.profiles.get(w.id)
        if (profile?.mood?.current === 'stressed' || profile?.mood?.current === 'frustrated') {
          const comforter = idleWorkers.find(iw => {
            if (iw.id === w.id) return false
            const s = this.workerStates.get(iw.id)
            return s?.state === 'at-desk'
          })
          if (comforter && Math.random() < 0.2) {
            this._startComforting(comforter, w)
          }
        }
      }
    }
  }

  // ── Personality-aware bubble content ─────────────────────────────────────

  _getBubbleContent(worker, activityType) {
    const profile = this.profiles?.get(worker.id)

    // Use catchphrases occasionally (30% chance)
    if (profile?.narrative?.catchphrases?.length && Math.random() < 0.3) {
      const phrases = profile.narrative.catchphrases
      return phrases[Math.floor(Math.random() * phrases.length)]
    }

    // Mood-based messages
    if (profile?.mood?.current === 'tired') {
      return TIRED_MESSAGES[Math.floor(Math.random() * TIRED_MESSAGES.length)]
    }

    // Activity-specific messages
    switch (activityType) {
      case 'comforting':
        return COMFORTING_MESSAGES[Math.floor(Math.random() * COMFORTING_MESSAGES.length)]
      case 'celebration':
        return CELEBRATION_MESSAGES[Math.floor(Math.random() * CELEBRATION_MESSAGES.length)]
      case 'pair_programming':
        return PAIR_PROG_MESSAGES[Math.floor(Math.random() * PAIR_PROG_MESSAGES.length)]
      default:
        return null // Falls back to existing message selection
    }
  }

  // ── Activity starters ─────────────────────────────────────────────────────

  _startWatercooler(workerId) {
    const tile = randomZoneTile('breakArea')
    if (!tile) return
    const ws = this.workerStates.get(workerId)
    ws.state = 'walking-to-zone'
    ws.data = { activity: 'watercooler' }
    this.renderer.moveWorkerTo(workerId, tile.col, tile.row)
    this._log(workerId, 'walking to watercooler')
  }

  _startDiscussion(workerId, partnerId) {
    const partnerWs = this.workerStates.get(partnerId)
    if (!partnerWs || partnerWs.state !== 'at-desk') return

    const topic = pick(DISCUSSION_MESSAGES)
    const ws = this.workerStates.get(workerId)
    ws.state = 'walking-to-person'
    ws.data = { activity: 'discussion', partnerId, topic }
    partnerWs.state = 'discussing'  // partner stays put, marks as occupied
    partnerWs.timer = 8000          // safety timer for partner

    this.renderer.moveWorkerToWorker(workerId, partnerId)
    this._log(workerId, `walking to discuss: ${topic}`)
  }

  _startQuickQuestion(workerId, partnerId) {
    const partnerWs = this.workerStates.get(partnerId)
    if (!partnerWs || partnerWs.state !== 'at-desk') return

    const topic = pick(DISCUSSION_MESSAGES)
    const dur = randBetween(2000, 3000)
    const ws = this.workerStates.get(workerId)
    ws.state = 'walking-to-person'
    ws.data = { activity: 'discussion', partnerId, topic }
    partnerWs.state = 'discussing'
    partnerWs.timer = dur + 4000

    this.renderer.moveWorkerToWorker(workerId, partnerId)
    this._log(workerId, 'quick question')
  }

  _startThinking(workerId) {
    const dur = 3000
    const ws = this.workerStates.get(workerId)
    ws.state = 'thinking'
    ws.timer = dur
    this.renderer.showThought(workerId, pick(THOUGHT_MESSAGES), dur)
    this._log(workerId, 'thinking')
  }

  _startMeeting(participants, topic) {
    const tile = randomZoneTile('meeting')
    if (!tile) return

    const ids = participants.map(w => w.id)
    for (const w of participants) {
      const ws = this.workerStates.get(w.id)
      ws.state = 'walking-to-zone'
      ws.data = { activity: 'meeting', meetingIds: ids, topic, bubbleShown: false }
      this.renderer.moveWorkerTo(w.id, tile.col, tile.row)
    }
    // Mark first participant as responsible for showing bubble
    const first = this.workerStates.get(ids[0])
    if (first) first.data.bubbleShown = false

    this._log(ids[0], `meeting started: ${topic}`)
  }

  _startPatrol(managerId, others) {
    if (!others.length) return
    const target = pick(others)
    const ws = this.workerStates.get(managerId)
    ws.state = 'walking-to-person'
    ws.data = { activity: 'patrol-visit' }
    this.renderer.moveWorkerToWorker(managerId, target.id)
    this._log(managerId, 'patrolling')
  }

  _startManagerMeeting(managerId) {
    const tile = randomZoneTile('meeting')
    if (!tile) return
    const ws = this.workerStates.get(managerId)
    ws.state = 'walking-to-zone'
    ws.data = { activity: 'manager-meeting' }
    this.renderer.moveWorkerTo(managerId, tile.col, tile.row)
    this._log(managerId, 'walking to meeting room')
  }

  _startPairProgramming(worker1, worker2) {
    const desk = this.deskMap?.get(worker1.id)
    if (!desk) return

    this.workerStates.set(worker1.id, { state: 'at-desk', data: { activity: 'pair_programming' }, timer: 8000 })
    this.workerStates.set(worker2.id, { state: 'walking-to-person', data: { targetId: worker1.id, activity: 'pair_programming' }, timer: 8000 })

    this._addBubble(worker1.id, this._getBubbleContent(worker1, 'pair_programming') || '一起寫 code')
    this._logActivity(`${worker1.name || worker1.id} 和 ${worker2.name || worker2.id} 開始結對編程`)
  }

  _startComforting(comforter, target) {
    this.workerStates.set(comforter.id, { state: 'walking-to-person', data: { targetId: target.id, activity: 'comforting' }, timer: 5000 })

    setTimeout(() => {
      this._addBubble(comforter.id, this._getBubbleContent(comforter, 'comforting') || '沒關係的')
    }, 2000)
    this._logActivity(`${comforter.name || comforter.id} 去關心 ${target.name || target.id}`)
  }

  _startCelebration(workers) {
    const zone = 'breakArea'
    for (const w of workers) {
      this.workerStates.set(w.id, { state: 'walking-to-zone', data: { zone, activity: 'celebration' }, timer: 6000 })
    }
    setTimeout(() => {
      for (const w of workers) {
        this._addBubble(w.id, this._getBubbleContent(w, 'celebration') || '🎉')
      }
    }, 3000)
    this._logActivity(`團隊慶祝！`)
  }

  // ── Backend event handlers ────────────────────────────────────────────────

  _onTaskAssigned(event) {
    for (const id of (event.workerIds || [])) {
      this._interruptWorker(id)
      const dur = 3000
      this.renderer.showSpeech(id, 'Starting task...', dur)
      const ws = this.workerStates.get(id)
      if (ws) {
        ws.state = 'thinking'
        ws.timer = dur
      }
      this._log(id, 'task assigned')
    }
  }

  _onTaskCompleted(event) {
    for (const id of (event.workerIds || [])) {
      this._interruptWorker(id)
      const dur = 3000
      this.renderer.showSpeech(id, '★ Done!', dur)
      this._log(id, 'task completed')
    }

    // Nearby workers react
    if (event.workerIds?.length) {
      const celebrant = event.workerIds[0]
      const nearby = (this._workers || [])
        .filter(w => w.id !== celebrant && this.workerStates.get(w.id)?.state === 'at-desk')
        .slice(0, 2)
      for (const w of nearby) {
        this.renderer.showSpeech(w.id, '👏', 1500)
      }
    }
  }

  _onReviewStarted(event) {
    const ids = event.workerIds || []
    if (ids.length < 2) return
    const [reviewerId, revieweeId] = ids
    this._interruptWorker(reviewerId)

    const ws = this.workerStates.get(reviewerId)
    if (ws) {
      ws.state = 'walking-to-person'
      ws.data = { activity: 'discussion', partnerId: revieweeId, topic: 'Code review' }
    }
    const partnerWs = this.workerStates.get(revieweeId)
    if (partnerWs) {
      partnerWs.state = 'discussing'
      partnerWs.timer = 10000
    }
    this.renderer.moveWorkerToWorker(reviewerId, revieweeId)
    this._log(reviewerId, 'review started')
  }

  _onReviewCompleted(event) {
    for (const id of (event.workerIds || [])) {
      this._interruptWorker(id)
      this.renderer.showSpeech(id, '✓ Review done', 2500)
      this._log(id, 'review completed')
    }
  }

  // ── Helpers ───────────────────────────────────────────────────────────────

  _workersWithState(state, status = null, tier = null) {
    return (this._workers || []).filter(w => {
      const ws = this.workerStates.get(w.id)
      if (!ws || ws.state !== state) return false
      if (status && w.status !== status) return false
      if (tier && (w.tier || 'engineer').toLowerCase() !== tier) return false
      return true
    })
  }

  _randomOther(workers, excludeId) {
    const candidates = workers.filter(w => {
      if (w.id === excludeId) return false
      const ws = this.workerStates.get(w.id)
      return ws && ws.state === 'at-desk'
    })
    if (!candidates.length) return null
    return pick(candidates)
  }

  _sample(arr, n) {
    const copy = [...arr]
    const result = []
    while (result.length < n && copy.length) {
      const i = Math.floor(Math.random() * copy.length)
      result.push(copy.splice(i, 1)[0])
    }
    return result
  }

  _interruptWorker(id) {
    const ws = this.workerStates.get(id)
    if (!ws) return
    this.renderer.clearWorkerBubbles(id)
    if (ws.state !== 'at-desk' && ws.state !== 'returning') {
      // Best-effort: stop any in-progress activity
      ws.state = 'at-desk'
      ws.data = {}
      ws.timer = 0
    }
  }

  _log(workerId, activity) {
    const worker = (this._workers || []).find(w => w.id === workerId)
    const name = worker?.name || workerId
    this.activityLog.push({ ts: Date.now(), name, activity })
    if (this.activityLog.length > 100) this.activityLog.shift()
  }

  _addBubble(workerId, text) {
    if (!text) return
    this.renderer.showSpeech(workerId, text, 4000)
  }

  _logActivity(activity) {
    this.activityLog.push({ ts: Date.now(), name: 'system', activity })
    if (this.activityLog.length > 100) this.activityLog.shift()
  }
}
