import {
  Microphone,
  MicrophoneSlash,
  VideoCamera,
  VideoCameraSlash,
  PhoneDisconnect,
} from "@phosphor-icons/react"
import {
  memo,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { VideoTile } from "@/components/video-tile"
import { LiveWaveform } from "@/components/ui/live-waveform"
import { Button } from "@/components/ui/button"
import { MetricsPanel } from "@/components/metrics-panel"
import type {
  ConnectionState,
  PeerConnectionStateValue,
  SelectedDevices,
} from "@/hooks/use-webrtc"
import type {
  DebugEvent,
  MetricPoint,
  StreamConnectionState,
  TranscriptMessage,
} from "@/hooks/use-room-stream"
import { formatLocalTime } from "@/lib/format-time"

interface RoomPageProps {
  localStream: MediaStream | null
  remoteStream: MediaStream | null
  roomId: string | null
  participantId: string | null
  connectionState: ConnectionState
  peerConnectionState: PeerConnectionStateValue
  streamStatus: StreamConnectionState
  selectedDevices: SelectedDevices
  debugEvents: DebugEvent[]
  transcript: TranscriptMessage[]
  metrics: MetricPoint[]
  latestByStage: Record<string, Record<string, MetricPoint>>
  onClearEvents: () => void
  isMicOn: boolean
  isCameraOn: boolean
  onToggleMic: () => void
  onToggleCamera: () => void
  onDisconnect: () => void
}

export function RoomPage({
  localStream,
  remoteStream,
  roomId,
  connectionState,
  peerConnectionState,
  streamStatus,
  selectedDevices,
  debugEvents,
  transcript,
  metrics,
  latestByStage,
  onClearEvents,
  isMicOn,
  isCameraOn,
  onToggleMic,
  onToggleCamera,
  onDisconnect,
}: RoomPageProps) {
  return (
    <div className="flex h-svh flex-col overflow-hidden bg-[#050505] text-white">
      <header className="flex items-center justify-between border-b border-[#1a1a1a] px-4 py-3">
        <div className="flex items-center gap-2">
          <StatusDot state={connectionState} />
          <span className="text-[10px] font-medium tracking-wider text-white/50 uppercase">
            Client {labelForState(connectionState)}
          </span>
        </div>
        <div className="flex items-center gap-3">
          {roomId && (
            <span
              className="max-w-[12rem] truncate text-[10px] tracking-wider text-white/30 uppercase"
              title={roomId}
            >
              Room {shortId(roomId)}
            </span>
          )}
          <span className="text-[10px] font-medium tracking-wider text-[#00d4aa]/60 uppercase">
            GO SFU DEBUG CONSOLE
          </span>
          <Controls
            isMicOn={isMicOn}
            isCameraOn={isCameraOn}
            onToggleMic={onToggleMic}
            onToggleCamera={onToggleCamera}
            onDisconnect={onDisconnect}
          />
        </div>
      </header>

      <main className="grid min-h-0 flex-1 grid-cols-1 grid-rows-[minmax(0,1fr)_16rem] gap-2 px-2 pt-2 pb-4 xl:grid-cols-[18rem_minmax(0,1fr)_22rem] xl:grid-rows-[minmax(0,1fr)_16rem]">
        <section className="grid min-h-0 gap-2 lg:grid-cols-2 xl:grid-cols-1">
          <Panel title="USER VIDEO" detail={isMicOn ? "MIC LIVE" : "MIC MUTED"}>
            <div className="space-y-3">
              <VideoTile stream={localStream} muted label="User" type="human" />
              <LiveWaveform
                active={isMicOn}
                height={42}
                barWidth={2}
                barGap={2}
                barColor="rgba(0, 212, 170, 0.8)"
                mode="static"
                fadeEdges
                sensitivity={1.5}
              />
            </div>
          </Panel>
          <Panel title="BOT VIDEO" detail={remoteStream ? "AGENT AUDIO" : "IDLE"}>
            <VideoTile stream={remoteStream} label="Agent" type="agent" />
          </Panel>
        </section>

        <Panel
          title="CONVERSATION"
          detail={`${transcript.length} ITEM${transcript.length === 1 ? "" : "S"}`}
          className="min-h-0"
          bodyClassName="min-h-0"
        >
          <TranscriptPanel transcript={transcript} connectionState={connectionState} />
        </Panel>

        <section className="flex min-h-0 flex-col gap-2">
          <SessionPanel
            roomId={roomId}
            connectionState={connectionState}
            peerConnectionState={peerConnectionState}
            streamStatus={streamStatus}
            selectedDevices={selectedDevices}
            isMicOn={isMicOn}
          />
          <MetricsPanel metrics={metrics} latestByStage={latestByStage} />
        </section>

        <Panel
          title="EVENTS"
          detail={`${debugEvents.length} CAPTURED`}
          className="min-h-0 xl:col-span-3"
          bodyClassName="min-h-0"
        >
          <EventLogPanel events={debugEvents} onClearEvents={onClearEvents} />
        </Panel>
      </main>
    </div>
  )
}

function Controls({
  isMicOn,
  isCameraOn,
  onToggleMic,
  onToggleCamera,
  onDisconnect,
}: Pick<
  RoomPageProps,
  | "isMicOn"
  | "isCameraOn"
  | "onToggleMic"
  | "onToggleCamera"
  | "onDisconnect"
>) {
  return (
    <div className="flex items-center gap-1.5">
      <Button
        onClick={onToggleMic}
        size="icon-lg"
        variant="outline"
        className={`border-[#1a1a1a] bg-[#0a0a0a] hover:border-[#00d4aa]/30 hover:bg-[#00d4aa]/5 ${isMicOn ? "text-[#00d4aa]" : "text-[#ff4444]"}`}
        aria-label={isMicOn ? "Mute microphone" : "Unmute microphone"}
      >
        {isMicOn ? (
          <Microphone weight="fill" />
        ) : (
          <MicrophoneSlash weight="fill" />
        )}
      </Button>
      <Button
        onClick={onToggleCamera}
        size="icon-lg"
        variant="outline"
        className={`border-[#1a1a1a] bg-[#0a0a0a] hover:border-[#00d4aa]/30 hover:bg-[#00d4aa]/5 ${isCameraOn ? "text-[#00d4aa]" : "text-[#ff4444]"}`}
        aria-label={isCameraOn ? "Turn off camera" : "Turn on camera"}
      >
        {isCameraOn ? (
          <VideoCamera weight="fill" />
        ) : (
          <VideoCameraSlash weight="fill" />
        )}
      </Button>
      <Button
        onClick={onDisconnect}
        size="icon-lg"
        variant="outline"
        className="border-[#1a1a1a] bg-[#0a0a0a] text-[#ff4444] hover:border-[#ff4444]/30 hover:bg-[#ff4444]/5"
        aria-label="Disconnect"
      >
        <PhoneDisconnect weight="fill" />
      </Button>
    </div>
  )
}

function Panel({
  title,
  detail,
  children,
  className = "",
  bodyClassName = "",
}: {
  title: string
  detail?: string
  children: ReactNode
  className?: string
  bodyClassName?: string
}) {
  return (
    <section
      className={`flex min-h-0 flex-col overflow-hidden border border-[#1a1a1a] bg-[#0a0a0a] ${className}`}
    >
      <div className="flex items-center justify-between border-b border-[#1a1a1a] px-3 py-2">
        <h2 className="text-[10px] font-medium tracking-wider text-[#00d4aa] uppercase">
          {title}
        </h2>
        {detail && <span className="text-[10px] tracking-wider text-white/30 uppercase">{detail}</span>}
      </div>
      <div
        className={`flex min-h-0 flex-1 flex-col overflow-hidden p-3 ${bodyClassName}`}
      >
        {children}
      </div>
    </section>
  )
}

function TranscriptPanel({
  transcript,
  connectionState,
}: {
  transcript: TranscriptMessage[]
  connectionState: ConnectionState
}) {
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: "end" })
  }, [transcript])

  if (transcript.length === 0) {
    return (
      <div className="flex h-full items-center justify-center border border-dashed border-[#1a1a1a] bg-[#050505] px-6 text-center">
        <div>
          <p className="text-[10px] font-medium tracking-wider text-white/40 uppercase">
            {connectionState === "connected" || connectionState === "connecting"
              ? "Waiting for live transcript"
              : "Connect a session to start the transcript"}
          </p>
          <p className="mt-2 text-[10px] tracking-wide text-white/20">
            User interim/final STT and agent responses will appear here in real
            time.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto">
        {transcript.map((item) => (
          <article
            key={item.id}
            className={`max-w-[82%] border px-3 py-2 ${
              item.speaker === "user"
                ? "self-start border-[#00d4aa]/20 bg-[#00d4aa]/5"
                : "self-end border-[#00d4aa]/30 bg-[#00d4aa]/10"
            }`}
          >
            <div className="mb-1.5 flex items-center gap-2 text-[9px] uppercase tracking-wider text-white/30">
              <span>{item.speaker === "user" ? "USER" : "BOT"}</span>
              {item.turn !== undefined && <span>TURN {item.turn}</span>}
              <span>{formatLocalTime(item.ts)}</span>
              {item.interim && <span className="text-[#00d4aa]">INTERIM</span>}
            </div>
            <p className="text-[11px] leading-5 text-white/70">{item.text}</p>
          </article>
        ))}
        <div ref={bottomRef} aria-hidden />
      </div>
    </div>
  )
}

function SessionPanel({
  roomId,
  connectionState,
  peerConnectionState,
  streamStatus,
  selectedDevices,
  isMicOn,
}: {
  roomId: string | null
  connectionState: ConnectionState
  peerConnectionState: PeerConnectionStateValue
  streamStatus: StreamConnectionState
  selectedDevices: SelectedDevices
  isMicOn: boolean
}) {
  const combinedState = useMemo(() => {
    if (
      connectionState === "failed" ||
      peerConnectionState === "failed" ||
      streamStatus === "failed"
    ) {
      return "failed"
    }
    if (
      connectionState === "connected" &&
      peerConnectionState === "connected" &&
      streamStatus === "connected"
    ) {
      return "connected"
    }
    if (
      connectionState === "connecting" ||
      peerConnectionState === "connecting" ||
      streamStatus === "connecting"
    ) {
      return "connecting"
    }
    return "idle"
  }, [connectionState, peerConnectionState, streamStatus])

  return (
    <Panel title="SESSION" detail="LIVE" className="shrink-0" bodyClassName="p-2">
      <div className="space-y-2.5">
        <InfoRow label="ROOM ID" value={shortId(roomId)} title={roomId ?? ""} />
        <InfoRow
          label="MICROPHONE"
          value={`${isMicOn ? "live" : "muted"} · ${selectedDevices.audioInput || "default"}`}
        />
        <div className="flex items-center justify-between gap-3">
          <span className="text-[9px] tracking-wider text-white/25 uppercase">
            Connection
          </span>
          <span className="flex items-center gap-2 text-[10px] font-medium tracking-wider text-white/60 uppercase">
            <StatusDot state={combinedState} />
            {combinedState}
          </span>
        </div>
      </div>
    </Panel>
  )
}

function InfoRow({
  label,
  value,
  title,
}: {
  label: string
  value: string
  title?: string
}) {
  return (
    <div>
      <div className="text-[9px] tracking-wider text-white/25">
        {label}
      </div>
      <div className="mt-0.5 truncate font-mono text-[10px] text-[#00d4aa]/70" title={title ?? value}>
        {value || "Unavailable"}
      </div>
    </div>
  )
}

function EventLogPanel({
  events,
  onClearEvents,
}: {
  events: DebugEvent[]
  onClearEvents: () => void
}) {
  const [category, setCategory] = useState("all")
  const [level, setLevel] = useState("all")
  const [query, setQuery] = useState("")
  const [paused, setPaused] = useState(false)
  const [pausedEvents, setPausedEvents] = useState<DebugEvent[]>([])
  const [categories, setCategories] = useState<string[]>([])
  const listRef = useRef<HTMLDivElement>(null)
  const pinnedToTopRef = useRef(true)
  const knownCategoriesRef = useRef(new Set<string>())
  const displayEvents = paused ? pausedEvents : events

  useEffect(() => {
    if (events.length === 0) {
      knownCategoriesRef.current.clear()
      setCategories([])
      return
    }

    let changed = false
    for (const event of events) {
      if (!knownCategoriesRef.current.has(event.category)) {
        knownCategoriesRef.current.add(event.category)
        changed = true
      }
    }
    if (changed) {
      setCategories(Array.from(knownCategoriesRef.current).sort())
    }
  }, [events])

  const filteredEvents = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    const matches = displayEvents.filter((event) => {
      if (category !== "all" && event.category !== category) return false
      if (level !== "all" && event.level !== level) return false
      if (!normalizedQuery) return true
      return `${event.type} ${event.message} ${JSON.stringify(event.attrs ?? {})}`
        .toLowerCase()
        .includes(normalizedQuery)
    })
    return matches.slice(-180).reverse()
  }, [category, displayEvents, level, query])

  useEffect(() => {
    if (paused || !pinnedToTopRef.current || !listRef.current) return
    listRef.current.scrollTop = 0
  }, [filteredEvents, paused])

  return (
    <div className="flex h-full min-h-0 flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="FILTER EVENTS..."
          className="h-6 min-w-48 border border-[#1a1a1a] bg-[#050505] px-2 font-mono text-[10px] text-[#00d4aa]/80 outline-none placeholder:text-white/20 focus:border-[#00d4aa]/30"
        />
        <select
          value={category}
          onChange={(event) => setCategory(event.target.value)}
          className="h-6 border border-[#1a1a1a] bg-[#050505] px-2 text-[10px] text-white/50 outline-none"
        >
          <option value="all">ALL CATEGORIES</option>
          {categories.map((item) => (
            <option key={item} value={item}>
              {item.toUpperCase()}
            </option>
          ))}
        </select>
        <select
          value={level}
          onChange={(event) => setLevel(event.target.value)}
          className="h-6 border border-[#1a1a1a] bg-[#050505] px-2 text-[10px] text-white/50 outline-none"
        >
          <option value="all">ALL LEVELS</option>
          <option value="debug">DEBUG</option>
          <option value="info">INFO</option>
          <option value="warn">WARN</option>
          <option value="error">ERROR</option>
        </select>
        <Button
          variant="outline"
          size="sm"
          className="h-6 border-[#1a1a1a] bg-[#050505] text-[10px] tracking-wider text-white/50 hover:border-[#00d4aa]/30 hover:text-[#00d4aa]"
          onClick={() => {
            if (paused) {
              setPaused(false)
              return
            }
            setPausedEvents(events)
            setPaused(true)
          }}
        >
          {paused ? "RESUME" : "PAUSE"}
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="h-6 border-[#1a1a1a] bg-[#050505] text-[10px] tracking-wider text-white/50 hover:border-[#00d4aa]/30 hover:text-[#00d4aa]"
          onClick={() => {
            knownCategoriesRef.current.clear()
            setCategories([])
            pinnedToTopRef.current = true
            onClearEvents()
          }}
        >
          CLEAR
        </Button>
      </div>

      <div
        ref={listRef}
        onScroll={() => {
          if (!listRef.current) return
          pinnedToTopRef.current = listRef.current.scrollTop < 8
        }}
        className="min-h-0 flex-1 overflow-y-auto border border-[#1a1a1a] bg-[#050505]"
      >
        {filteredEvents.length === 0 ? (
          <div className="flex h-full items-center justify-center text-[10px] tracking-wider text-white/25 uppercase">
            No events match the current filters.
          </div>
        ) : (
          <div className="divide-y divide-[#1a1a1a]">
            {filteredEvents.map((event) => (
              <EventRow event={event} key={event.id} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

const EventRow = memo(function EventRow({ event }: { event: DebugEvent }) {
  return (
    <div className="grid grid-cols-[4rem_4rem_9rem_minmax(0,1fr)] gap-3 px-3 py-1.5 text-[10px]">
      <span className="font-mono text-white/25">{formatLocalTime(event.ts)}</span>
      <span className={levelClass(event.level)}>{event.level.toUpperCase()}</span>
      <span className="truncate font-mono text-[#00d4aa]/50" title={event.type}>
        {event.type}
      </span>
      <div className="min-w-0">
        <div className="truncate text-white/50">{event.message}</div>
        {event.attrs && (
          <div className="mt-0.5 truncate font-mono text-[9px] text-white/20">
            {attrSummary(event.attrs)}
          </div>
        )}
      </div>
    </div>
  )
})

function StatusDot({ state }: { state: string }) {
  const tone =
    state.includes("failed") ||
    state.includes("closed") ||
    state.includes("disconnected")
      ? "bg-[#ff4444] shadow-[0_0_6px_rgba(255,68,68,0.6)]"
      : state.includes("connected") || state === "on" || state === "enabled"
        ? "bg-[#00d4aa] shadow-[0_0_6px_rgba(0,212,170,0.6)]"
        : state.includes("connecting") || state.includes("checking")
          ? "bg-[#ffaa00] shadow-[0_0_6px_rgba(255,170,0,0.6)]"
          : "bg-white/20"

  return <span className={`h-1.5 w-1.5 rounded-full ${tone}`} />
}

function labelForState(state: string) {
  if (state === "idle") return "idle"
  if (state === "connected") return "connected"
  if (state === "connecting") return "connecting"
  if (state === "disconnected") return "disconnected"
  if (state === "failed") return "failed"
  return state
}

function shortId(value: string | null) {
  if (!value) return "Unavailable"
  if (value.length <= 12) return value
  return `${value.slice(0, 8)}...${value.slice(-4)}`
}

function levelClass(level: DebugEvent["level"]) {
  switch (level) {
    case "error":
      return "font-medium text-[#ff4444]"
    case "warn":
      return "font-medium text-[#ffaa00]"
    case "debug":
      return "text-white/30"
    default:
      return "text-[#00d4aa]"
  }
}

function attrSummary(attrs: Record<string, unknown>) {
  return Object.entries(attrs)
    .filter(([key]) => !["room", "text"].includes(key))
    .slice(0, 6)
    .map(([key, value]) => `${key}=${formatAttrValue(value)}`)
    .join("  ")
}

function formatAttrValue(value: unknown) {
  if (typeof value === "string") return value
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value)
  }
  return JSON.stringify(value)
}
