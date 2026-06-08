import {
  Microphone,
  MicrophoneSlash,
  VideoCamera,
  VideoCameraSlash,
  PhoneDisconnect,
} from "@phosphor-icons/react"
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import { VideoTile } from "@/components/video-tile"
import { LiveWaveform } from "@/components/ui/live-waveform"
import { Button } from "@/components/ui/button"
import type {
  ConnectionState,
  IceConnectionStateValue,
  PeerConnectionStateValue,
  SelectedDevices,
} from "@/hooks/use-webrtc"
import type {
  DebugConnectionState,
  DebugEvent,
  TranscriptMessage,
} from "@/hooks/use-session-debug"

interface RoomPageProps {
  localStream: MediaStream | null
  remoteStream: MediaStream | null
  roomId: string | null
  participantId: string | null
  connectionState: ConnectionState
  peerConnectionState: PeerConnectionStateValue
  iceConnectionState: IceConnectionStateValue
  debugStatus: DebugConnectionState
  devices: MediaDeviceInfo[]
  selectedDevices: SelectedDevices
  debugEvents: DebugEvent[]
  transcript: TranscriptMessage[]
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
  participantId,
  connectionState,
  peerConnectionState,
  iceConnectionState,
  debugStatus,
  devices,
  selectedDevices,
  debugEvents,
  transcript,
  onClearEvents,
  isMicOn,
  isCameraOn,
  onToggleMic,
  onToggleCamera,
  onDisconnect,
}: RoomPageProps) {
  return (
    <div className="flex h-svh flex-col overflow-hidden bg-[#050506] text-white">
      <header className="flex items-center justify-between border-b border-white/10 px-4 py-3">
        <div className="flex items-center gap-2">
          <StatusDot state={connectionState} />
          <span className="text-xs font-medium text-white/70">
            Client {labelForState(connectionState)}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs font-medium tracking-wide text-white/40">
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

      <main className="grid min-h-0 flex-1 grid-cols-1 grid-rows-[minmax(0,1fr)_16rem] gap-2 px-2 pt-2 pb-4 xl:grid-cols-[18rem_minmax(0,1fr)_20rem] xl:grid-rows-[minmax(0,1fr)_16rem]">
        <section className="grid min-h-0 gap-2 lg:grid-cols-2 xl:grid-cols-1">
          <Panel title="User Video" detail={isMicOn ? "Mic live" : "Mic muted"}>
            <div className="space-y-3">
              <VideoTile stream={localStream} muted label="User" type="human" />
              <LiveWaveform
                active={isMicOn}
                height={42}
                barWidth={2}
                barGap={2}
                barColor="rgba(168, 85, 247, 0.8)"
                mode="static"
                fadeEdges
                sensitivity={1.5}
              />
            </div>
          </Panel>
          <Panel title="Bot Video" detail={remoteStream ? "Agent audio" : "Idle"}>
            <VideoTile stream={remoteStream} label="Agent" type="agent" />
          </Panel>
        </section>

        <Panel
          title="Conversation"
          detail={`${transcript.length} transcript item${transcript.length === 1 ? "" : "s"}`}
          className="min-h-0"
          bodyClassName="min-h-0"
        >
          <TranscriptPanel transcript={transcript} connectionState={connectionState} />
        </Panel>

        <SessionPanel
          roomId={roomId}
          participantId={participantId}
          connectionState={connectionState}
          peerConnectionState={peerConnectionState}
          iceConnectionState={iceConnectionState}
          debugStatus={debugStatus}
          devices={devices}
          selectedDevices={selectedDevices}
          isMicOn={isMicOn}
          isCameraOn={isCameraOn}
        />

        <Panel
          title="Events"
          detail={`${debugEvents.length} captured`}
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
    <div className="flex items-center gap-2">
      <Button
        onClick={onToggleMic}
        size="icon-lg"
        variant={isMicOn ? "outline" : "destructive"}
        className="rounded-full border-white/10"
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
        variant={isCameraOn ? "outline" : "destructive"}
        className="rounded-full border-white/10"
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
        variant="destructive"
        className="rounded-full"
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
      className={`flex min-h-0 flex-col overflow-hidden rounded-xl border border-white/10 bg-white/[0.035] shadow-2xl shadow-black/30 ${className}`}
    >
      <div className="flex items-center justify-between border-b border-white/10 px-3 py-2">
        <h2 className="text-xs font-semibold tracking-wide text-white/80">
          {title}
        </h2>
        {detail && <span className="text-[11px] text-white/35">{detail}</span>}
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
      <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-white/10 bg-black/20 px-6 text-center">
        <div>
          <p className="text-xs font-medium text-white/70">
            {connectionState === "connected" || connectionState === "connecting"
              ? "Waiting for live transcript"
              : "Connect a session to start the transcript"}
          </p>
          <p className="mt-2 text-xs text-white/35">
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
            className={`max-w-[82%] rounded-2xl border px-3 py-2 ${
              item.speaker === "user"
                ? "self-start border-violet-400/20 bg-violet-500/10"
                : "self-end border-emerald-400/20 bg-emerald-500/10"
            }`}
          >
            <div className="mb-1.5 flex items-center gap-2 text-[10px] uppercase tracking-wide text-white/35">
              <span>{item.speaker === "user" ? "User" : "Bot"}</span>
              {item.turn !== undefined && <span>Turn {item.turn}</span>}
              <span>{formatTime(item.ts)}</span>
              {item.interim && <span className="text-violet-200">Interim</span>}
            </div>
            <p className="text-xs leading-5 text-white/85">{item.text}</p>
          </article>
        ))}
        <div ref={bottomRef} aria-hidden />
      </div>
    </div>
  )
}

function SessionPanel({
  roomId,
  participantId,
  connectionState,
  peerConnectionState,
  iceConnectionState,
  debugStatus,
  devices,
  selectedDevices,
  isMicOn,
  isCameraOn,
}: Pick<
  RoomPageProps,
  | "roomId"
  | "participantId"
  | "connectionState"
  | "peerConnectionState"
  | "iceConnectionState"
  | "debugStatus"
  | "devices"
  | "selectedDevices"
  | "isMicOn"
  | "isCameraOn"
>) {
  const deviceCounts = useMemo(
    () => ({
      audioinput: devices.filter((device) => device.kind === "audioinput").length,
      videoinput: devices.filter((device) => device.kind === "videoinput").length,
      audiooutput: devices.filter((device) => device.kind === "audiooutput").length,
    }),
    [devices]
  )

  return (
    <Panel title="Session" detail="Live state" className="min-h-0" bodyClassName="min-h-0">
      <div className="h-full min-h-0 overflow-y-auto">
        <div className="space-y-4">
          <StatusRow label="Client" value={labelForState(connectionState)} />
          <StatusRow label="Agent" value={agentStateLabel(peerConnectionState)} />
          <StatusRow label="Debug WS" value={labelForState(debugStatus)} />
          <Divider />
          <InfoRow label="Microphone" value={selectedDevices.audioInput} />
          <InfoRow label="Camera" value={selectedDevices.videoInput} />
          <InfoRow label="Audio Output" value={selectedDevices.audioOutput} />
          <InfoRow
            label="Device Counts"
            value={`${deviceCounts.audioinput} mic / ${deviceCounts.videoinput} camera / ${deviceCounts.audiooutput} output`}
          />
          <Divider />
          <InfoRow label="Transport" value="WebRTC + WebSocket debug" />
          <InfoRow label="Session ID" value={shortId(roomId)} title={roomId ?? ""} />
          <InfoRow
            label="Participant ID"
            value={shortId(participantId)}
            title={participantId ?? ""}
          />
          <InfoRow label="Connection" value={peerConnectionState} />
          <InfoRow label="ICE" value={iceConnectionState} />
          <InfoRow label="Mic Track" value={isMicOn ? "enabled" : "muted"} />
          <InfoRow label="Camera Track" value={isCameraOn ? "enabled" : "off"} />
        </div>
      </div>
    </Panel>
  )
}

function StatusRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="text-xs text-white/45">{label}</span>
      <span className="flex items-center gap-2 text-xs font-medium text-white/80">
        <StatusDot state={value} />
        {value}
      </span>
    </div>
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
      <div className="text-[11px] uppercase tracking-wide text-white/30">
        {label}
      </div>
      <div className="mt-1 truncate text-xs text-white/75" title={title ?? value}>
        {value || "Unavailable"}
      </div>
    </div>
  )
}

function Divider() {
  return <div className="h-px bg-white/10" />
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
  const displayEvents = paused ? pausedEvents : events

  const categories = useMemo(
    () => Array.from(new Set(events.map((event) => event.category))).sort(),
    [events]
  )

  const filteredEvents = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase()
    return displayEvents
      .filter((event) => category === "all" || event.category === category)
      .filter((event) => level === "all" || event.level === level)
      .filter((event) => {
        if (!normalizedQuery) return true
        return `${event.type} ${event.message} ${JSON.stringify(event.attrs ?? {})}`
          .toLowerCase()
          .includes(normalizedQuery)
      })
      .slice(-180)
  }, [category, displayEvents, level, query])

  return (
    <div className="flex h-full min-h-0 flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Filter events"
          className="h-7 min-w-48 rounded-md border border-white/10 bg-black/30 px-2 text-xs text-white/80 outline-none placeholder:text-white/25 focus:border-white/25"
        />
        <select
          value={category}
          onChange={(event) => setCategory(event.target.value)}
          className="h-7 rounded-md border border-white/10 bg-black/30 px-2 text-xs text-white/70 outline-none"
        >
          <option value="all">All categories</option>
          {categories.map((item) => (
            <option key={item} value={item}>
              {item}
            </option>
          ))}
        </select>
        <select
          value={level}
          onChange={(event) => setLevel(event.target.value)}
          className="h-7 rounded-md border border-white/10 bg-black/30 px-2 text-xs text-white/70 outline-none"
        >
          <option value="all">All levels</option>
          <option value="debug">Debug</option>
          <option value="info">Info</option>
          <option value="warn">Warn</option>
          <option value="error">Error</option>
        </select>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            if (paused) {
              setPaused(false)
              return
            }
            setPausedEvents(events)
            setPaused(true)
          }}
        >
          {paused ? "Resume" : "Pause"}
        </Button>
        <Button variant="outline" size="sm" onClick={onClearEvents}>
          Clear
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto rounded-lg border border-white/10 bg-black/30">
        {filteredEvents.length === 0 ? (
          <div className="flex h-full items-center justify-center text-xs text-white/35">
            No events match the current filters.
          </div>
        ) : (
          <div className="divide-y divide-white/5">
            {filteredEvents.map((event) => (
              <EventRow event={event} key={event.id} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function EventRow({ event }: { event: DebugEvent }) {
  return (
    <div className="grid grid-cols-[4rem_5rem_9rem_minmax(0,1fr)] gap-3 px-3 py-2 text-xs">
      <span className="font-mono text-white/35">{formatTime(event.ts)}</span>
      <span className={levelClass(event.level)}>{event.level}</span>
      <span className="truncate font-mono text-white/45" title={event.type}>
        {event.type}
      </span>
      <div className="min-w-0">
        <div className="truncate text-white/75">{event.message}</div>
        {event.attrs && (
          <div className="mt-1 truncate font-mono text-[11px] text-white/30">
            {attrSummary(event.attrs)}
          </div>
        )}
      </div>
    </div>
  )
}

function StatusDot({ state }: { state: string }) {
  const tone =
    state.includes("failed") ||
    state.includes("closed") ||
    state.includes("disconnected")
      ? "bg-red-400 shadow-[0_0_8px_rgba(248,113,113,0.7)]"
      : state.includes("connected") || state === "on" || state === "enabled"
        ? "bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.8)]"
        : state.includes("connecting") || state.includes("checking")
          ? "bg-amber-300 shadow-[0_0_8px_rgba(252,211,77,0.7)]"
          : "bg-white/30"

  return <span className={`h-2 w-2 rounded-full ${tone}`} />
}

function labelForState(state: string) {
  if (state === "idle") return "idle"
  if (state === "connected") return "connected"
  if (state === "connecting") return "connecting"
  if (state === "disconnected") return "disconnected"
  if (state === "failed") return "failed"
  return state
}

function agentStateLabel(peerState: PeerConnectionStateValue) {
  if (peerState === "connected") return "connected"
  if (peerState === "connecting" || peerState === "new") return "connecting"
  if (peerState === "failed" || peerState === "closed") return "failed"
  return "idle"
}

function shortId(value: string | null) {
  if (!value) return "Unavailable"
  if (value.length <= 12) return value
  return `${value.slice(0, 8)}...${value.slice(-4)}`
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value))
}

function levelClass(level: DebugEvent["level"]) {
  switch (level) {
    case "error":
      return "font-semibold text-red-300"
    case "warn":
      return "font-semibold text-amber-200"
    case "debug":
      return "text-sky-200"
    default:
      return "text-emerald-200"
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
