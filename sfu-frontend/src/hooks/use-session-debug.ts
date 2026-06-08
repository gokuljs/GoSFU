import { useCallback, useEffect, useMemo, useState } from "react"

const MAX_DEBUG_EVENTS = 600

export type DebugLevel = "debug" | "info" | "warn" | "error"
export type DebugConnectionState =
  | "idle"
  | "connecting"
  | "connected"
  | "disconnected"
  | "failed"

export interface DebugEvent {
  id: string
  ts: string
  room: string
  type: string
  category: string
  level: DebugLevel
  message: string
  source: string
  turn?: number
  attrs?: Record<string, unknown>
}

export interface TranscriptMessage {
  id: string
  speaker: "user" | "agent"
  text: string
  ts: string
  turn?: number
  interim: boolean
}

export interface UseSessionDebugReturn {
  events: DebugEvent[]
  transcript: TranscriptMessage[]
  status: DebugConnectionState
  addLocalEvent: (
    type: string,
    message: string,
    attrs?: Record<string, unknown>,
    level?: DebugLevel
  ) => void
  clearEvents: () => void
}

function websocketUrl(baseUrl: string, roomId: string) {
  const url = new URL(baseUrl)
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:"
  url.pathname = `/room/${roomId}/debug`
  url.search = ""
  return url.toString()
}

function eventId() {
  if (window.crypto?.randomUUID) return window.crypto.randomUUID()
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function stringAttr(event: DebugEvent, key: string) {
  const value = event.attrs?.[key]
  return typeof value === "string" ? value.trim() : ""
}

function upsertTranscript(
  messages: Map<string, TranscriptMessage>,
  event: DebugEvent,
  speaker: TranscriptMessage["speaker"],
  text: string,
  interim: boolean
) {
  if (!text) return
  const turn = event.turn ?? Number(event.attrs?.turn)
  const id = `${speaker}-${turn || event.id}`
  messages.set(id, {
    id,
    speaker,
    text,
    ts: event.ts,
    turn: Number.isFinite(turn) ? turn : undefined,
    interim,
  })
}

function deriveTranscript(events: DebugEvent[]) {
  const messages = new Map<string, TranscriptMessage>()

  for (const event of events) {
    if (event.type === "pipeline.transcript.interim") {
      upsertTranscript(messages, event, "user", stringAttr(event, "text"), true)
      continue
    }
    if (event.type === "pipeline.turn.ready") {
      upsertTranscript(messages, event, "user", stringAttr(event, "text"), false)
      continue
    }
    if (event.type === "pipeline.stt.result") {
      upsertTranscript(messages, event, "user", stringAttr(event, "text"), false)
      continue
    }
    if (
      event.type === "pipeline.llm.complete" ||
      event.type === "pipeline.agent.response_done" ||
      event.type === "pipeline.agent.response_started"
    ) {
      upsertTranscript(messages, event, "agent", stringAttr(event, "text"), false)
    }
  }

  return Array.from(messages.values()).sort(
    (a, b) => new Date(a.ts).getTime() - new Date(b.ts).getTime()
  )
}

export function useSessionDebug(
  roomId: string | null,
  sfuUrl: string
): UseSessionDebugReturn {
  const [events, setEvents] = useState<DebugEvent[]>([])
  const [status, setStatus] = useState<DebugConnectionState>("idle")

  const addEvent = useCallback((event: DebugEvent) => {
    setEvents((current) => [...current, event].slice(-MAX_DEBUG_EVENTS))
  }, [])

  const addLocalEvent = useCallback(
    (
      type: string,
      message: string,
      attrs: Record<string, unknown> = {},
      level: DebugLevel = "info"
    ) => {
      addEvent({
        id: eventId(),
        ts: new Date().toISOString(),
        room: roomId ?? "local",
        type,
        category: type.split(".")[0] || "client",
        level,
        message,
        source: "client",
        attrs,
      })
    },
    [addEvent, roomId]
  )

  useEffect(() => {
    if (!roomId) {
      const timeout = window.setTimeout(() => setStatus("idle"), 0)
      return () => window.clearTimeout(timeout)
    }

    const timeout = window.setTimeout(() => setStatus("connecting"), 0)
    const ws = new WebSocket(websocketUrl(sfuUrl, roomId))

    ws.onopen = () => {
      window.clearTimeout(timeout)
      setStatus("connected")
      addLocalEvent("client.websocket.connected", "Client WebSocket connected")
    }
    ws.onmessage = (message) => {
      try {
        addEvent(JSON.parse(message.data) as DebugEvent)
      } catch {
        addLocalEvent(
          "client.websocket.parse_failed",
          "Failed to parse debug event",
          { payload_type: typeof message.data },
          "warn"
        )
      }
    }
    ws.onerror = () => {
      window.clearTimeout(timeout)
      setStatus("failed")
      addLocalEvent("client.websocket.error", "Client WebSocket error", {}, "error")
    }
    ws.onclose = () => {
      window.clearTimeout(timeout)
      setStatus((current) => (current === "failed" ? "failed" : "disconnected"))
      addLocalEvent(
        "client.websocket.disconnected",
        "Client WebSocket disconnected"
      )
    }

    return () => {
      window.clearTimeout(timeout)
      ws.close()
    }
  }, [addEvent, addLocalEvent, roomId, sfuUrl])

  const transcript = useMemo(() => deriveTranscript(events), [events])

  return {
    events,
    transcript,
    status,
    addLocalEvent,
    clearEvents: useCallback(() => setEvents([]), []),
  }
}
