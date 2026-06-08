import { useCallback, useEffect, useState } from "react"

export type TranscriptConnectionState =
  | "idle"
  | "connecting"
  | "connected"
  | "disconnected"
  | "failed"

export interface TranscriptMessage {
  id: string
  speaker: "user" | "agent"
  text: string
  ts: string
  turn?: number
  interim: boolean
}

interface TranscriptUpdate {
  speaker: "user" | "agent"
  text: string
  final: boolean
  turn: number
  ts: string
}

export interface UseTranscriptReturn {
  transcript: TranscriptMessage[]
  status: TranscriptConnectionState
}

function websocketUrl(baseUrl: string, roomId: string) {
  const url = new URL(baseUrl)
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:"
  url.pathname = `/room/${roomId}/transcript`
  url.search = ""
  return url.toString()
}

function upsertMessage(
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

export function useTranscript(
  roomId: string | null,
  sfuUrl: string
): UseTranscriptReturn {
  const [transcript, setTranscript] = useState<TranscriptMessage[]>([])
  const [status, setStatus] = useState<TranscriptConnectionState>("idle")

  const applyUpdate = useCallback((update: TranscriptUpdate) => {
    setTranscript((current) => {
      const messages = new Map(current.map((item) => [item.id, item]))
      upsertMessage(messages, update)
      return Array.from(messages.values()).sort(
        (a, b) => new Date(a.ts).getTime() - new Date(b.ts).getTime()
      )
    })
  }, [])

  useEffect(() => {
    if (!roomId) {
      const timeout = window.setTimeout(() => {
        setStatus("idle")
        setTranscript([])
      }, 0)
      return () => window.clearTimeout(timeout)
    }

    const timeout = window.setTimeout(() => setStatus("connecting"), 0)
    const ws = new WebSocket(websocketUrl(sfuUrl, roomId))

    ws.onopen = () => {
      window.clearTimeout(timeout)
      setStatus("connected")
    }
    ws.onmessage = (message) => {
      try {
        applyUpdate(JSON.parse(message.data) as TranscriptUpdate)
      } catch {
        // Ignore malformed transcript updates.
      }
    }
    ws.onerror = () => {
      window.clearTimeout(timeout)
      setStatus("failed")
    }
    ws.onclose = () => {
      window.clearTimeout(timeout)
      setStatus((current) => (current === "failed" ? "failed" : "disconnected"))
    }

    return () => {
      window.clearTimeout(timeout)
      ws.close()
    }
  }, [applyUpdate, roomId, sfuUrl])

  return { transcript, status }
}
