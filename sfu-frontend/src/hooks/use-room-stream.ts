import { useCallback, useEffect, useMemo, useRef, useState } from "react"

const MAX_DEBUG_EVENTS = 600
const DEBUG_EVENT_FLUSH_MS = 300
const MAX_METRIC_POINTS = 400
const METRICS_DEBUG = import.meta.env.DEV

export type StreamConnectionState =
  | "idle"
  | "connecting"
  | "connected"
  | "disconnected"
  | "failed"

export type DebugLevel = "debug" | "info" | "warn" | "error"

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

export interface MetricPoint {
  id: string
  ts: string
  turn: number
  stage: "stt" | "llm" | "tts"
  name: string
  value: number
  unit: string
}

interface StreamEnvelope {
  channel: "transcript" | "debug" | "metrics"
  data: unknown
}

interface TranscriptUpdate {
  speaker: "user" | "agent"
  text: string
  final: boolean
  turn: number
  ts: string
}

interface MetricUpdate {
  id: string
  ts: string
  room: string
  turn: number
  stage: "stt" | "llm" | "tts"
  name: string
  value: number
  unit: string
}

export interface UseRoomStreamReturn {
  status: StreamConnectionState
  transcript: TranscriptMessage[]
  debugEvents: DebugEvent[]
  metrics: MetricPoint[]
  latestByStage: Record<string, Record<string, MetricPoint>>
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
  url.pathname = `/room/${roomId}/stream`
  url.search = ""
  return url.toString()
}

function eventId() {
  if (window.crypto?.randomUUID) return window.crypto.randomUUID()
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function upsertTranscript(
  messages: Map<string, TranscriptMessage>,
  update: TranscriptUpdate
) {
  const text = update.text.trim()
  if (!text) return

  const id = `${update.speaker}-${update.turn}`
  messages.set(id, {
    id,
    speaker: update.speaker,
    text,
    ts: update.ts,
    turn: update.turn,
    interim: !update.final,
  })
}

export function useRoomStream(
  roomId: string | null,
  sfuUrl: string
): UseRoomStreamReturn {
  const [transcript, setTranscript] = useState<TranscriptMessage[]>([])
  const [debugEvents, setDebugEvents] = useState<DebugEvent[]>([])
  const [metrics, setMetrics] = useState<MetricPoint[]>([])
  const [status, setStatus] = useState<StreamConnectionState>("idle")
  const pendingDebugEventsRef = useRef<DebugEvent[]>([])
  const flushTimerRef = useRef<number | null>(null)

  const flushDebugEvents = useCallback(() => {
    if (pendingDebugEventsRef.current.length === 0) return
    const batch = pendingDebugEventsRef.current
    pendingDebugEventsRef.current = []
    setDebugEvents((current) => [...current, ...batch].slice(-MAX_DEBUG_EVENTS))
  }, [])

  const scheduleDebugFlush = useCallback(() => {
    if (flushTimerRef.current !== null) return
    flushTimerRef.current = window.setTimeout(() => {
      flushTimerRef.current = null
      flushDebugEvents()
    }, DEBUG_EVENT_FLUSH_MS)
  }, [flushDebugEvents])

  const addDebugEvent = useCallback(
    (event: DebugEvent) => {
      pendingDebugEventsRef.current.push(event)
      scheduleDebugFlush()
    },
    [scheduleDebugFlush]
  )

  const clearPendingDebugEvents = useCallback(() => {
    pendingDebugEventsRef.current = []
    if (flushTimerRef.current !== null) {
      window.clearTimeout(flushTimerRef.current)
      flushTimerRef.current = null
    }
  }, [])

  const addLocalEvent = useCallback(
    (
      type: string,
      message: string,
      attrs: Record<string, unknown> = {},
      level: DebugLevel = "info"
    ) => {
      addDebugEvent({
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
    [addDebugEvent, roomId]
  )

  const applyTranscript = useCallback((update: TranscriptUpdate) => {
    setTranscript((current) => {
      const messages = new Map(current.map((item) => [item.id, item]))
      upsertTranscript(messages, update)
      return Array.from(messages.values()).sort(
        (a, b) => new Date(a.ts).getTime() - new Date(b.ts).getTime()
      )
    })
  }, [])

  const applyMetric = useCallback((update: MetricUpdate) => {
    if (METRICS_DEBUG) {
      console.debug("[stream] received", "metrics", update.name, update.value)
    }
    setMetrics((current) =>
      [
        ...current,
        {
          id: update.id,
          ts: update.ts,
          turn: update.turn,
          stage: update.stage,
          name: update.name,
          value: update.value,
          unit: update.unit,
        },
      ].slice(-MAX_METRIC_POINTS)
    )
  }, [])

  useEffect(() => {
    return () => {
      if (flushTimerRef.current !== null) {
        window.clearTimeout(flushTimerRef.current)
        flushTimerRef.current = null
      }
      flushDebugEvents()
    }
  }, [flushDebugEvents])

  useEffect(() => {
    if (!roomId) {
      const timeout = window.setTimeout(() => {
        clearPendingDebugEvents()
        setStatus("idle")
        setTranscript([])
        setDebugEvents([])
        setMetrics([])
      }, 0)
      return () => window.clearTimeout(timeout)
    }

    const timeout = window.setTimeout(() => setStatus("connecting"), 0)
    const ws = new WebSocket(websocketUrl(sfuUrl, roomId))

    ws.onopen = () => {
      window.clearTimeout(timeout)
      setStatus("connected")
      addLocalEvent("client.websocket.connected", "Client room stream connected")
    }

    ws.onmessage = (message) => {
      try {
        const envelope = JSON.parse(message.data) as StreamEnvelope
        if (METRICS_DEBUG) {
          console.debug("[stream] received", envelope.channel)
        }
        switch (envelope.channel) {
          case "transcript":
            applyTranscript(envelope.data as TranscriptUpdate)
            break
          case "debug":
            addDebugEvent(envelope.data as DebugEvent)
            break
          case "metrics":
            applyMetric(envelope.data as MetricUpdate)
            break
        }
      } catch {
        addLocalEvent(
          "client.websocket.parse_failed",
          "Failed to parse stream message",
          { payload_type: typeof message.data },
          "warn"
        )
      }
    }

    ws.onerror = () => {
      window.clearTimeout(timeout)
      setStatus("failed")
      addLocalEvent("client.websocket.error", "Client room stream error", {}, "error")
    }

    ws.onclose = () => {
      window.clearTimeout(timeout)
      setStatus((current) => (current === "failed" ? "failed" : "disconnected"))
      addLocalEvent("client.websocket.disconnected", "Client room stream disconnected")
    }

    return () => {
      window.clearTimeout(timeout)
      ws.close()
      if (flushTimerRef.current !== null) {
        window.clearTimeout(flushTimerRef.current)
        flushTimerRef.current = null
      }
      flushDebugEvents()
    }
  }, [
    addDebugEvent,
    addLocalEvent,
    applyMetric,
    applyTranscript,
    clearPendingDebugEvents,
    flushDebugEvents,
    roomId,
    sfuUrl,
  ])

  const latestByStage = useMemo(() => {
    const result: Record<string, Record<string, MetricPoint>> = {
      stt: {},
      llm: {},
      tts: {},
    }
    for (const point of metrics) {
      if (!result[point.stage]) result[point.stage] = {}
      result[point.stage][point.name] = point
    }
    return result
  }, [metrics])

  return {
    status,
    transcript,
    debugEvents,
    metrics,
    latestByStage,
    addLocalEvent,
    clearEvents: useCallback(() => {
      clearPendingDebugEvents()
      setDebugEvents([])
    }, [clearPendingDebugEvents]),
  }
}

// Back-compat type aliases
export type DebugConnectionState = StreamConnectionState
