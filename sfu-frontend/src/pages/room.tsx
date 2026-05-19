import {
  Microphone,
  MicrophoneSlash,
  VideoCamera,
  VideoCameraSlash,
  PhoneDisconnect,
} from "@phosphor-icons/react"
import { VideoTile } from "@/components/video-tile"
import { LiveWaveform } from "@/components/ui/live-waveform"
import { Button } from "@/components/ui/button"
import type { ConnectionState } from "@/hooks/use-webrtc"

interface RoomPageProps {
  localStream: MediaStream | null
  remoteStream: MediaStream | null
  connectionState: ConnectionState
  isMicOn: boolean
  isCameraOn: boolean
  onToggleMic: () => void
  onToggleCamera: () => void
  onDisconnect: () => void
}

export function RoomPage({
  localStream,
  remoteStream,
  connectionState,
  isMicOn,
  isCameraOn,
  onToggleMic,
  onToggleCamera,
  onDisconnect,
}: RoomPageProps) {
  return (
    <div className="flex min-h-svh flex-col bg-black">
      {/* Header */}
      <header className="flex items-center justify-between border-b border-white/5 px-4 py-3 sm:px-6">
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 rounded-full bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.6)]" />
          <span className="text-xs font-medium text-white/60">
            {connectionState === "connected"
              ? "Connected"
              : connectionState === "connecting"
                ? "Connecting..."
                : "Disconnected"}
          </span>
        </div>
        <span className="text-xs font-medium tracking-wide text-white/40">
          GO SFU
        </span>
      </header>

      {/* Video Grid */}
      <main className="flex flex-1 items-center justify-center p-3 sm:p-6">
        <div className="grid w-full max-w-5xl grid-cols-1 gap-3 sm:gap-4 md:grid-cols-2">
          <VideoTile stream={localStream} muted label="You" type="human" />
          <VideoTile stream={remoteStream} label="Agent" type="agent" />
        </div>
      </main>

      {/* Waveform indicator */}
      <div className="flex justify-center px-4 pb-2">
        <div className="w-full max-w-md">
          <LiveWaveform
            active={isMicOn}
            height={40}
            barWidth={2}
            barGap={2}
            barColor="rgba(255, 255, 255, 0.4)"
            mode="static"
            fadeEdges={true}
            sensitivity={1.5}
          />
        </div>
      </div>

      {/* Controls Toolbar */}
      <footer className="flex items-center justify-center gap-3 border-t border-white/5 px-4 py-4">
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
      </footer>
    </div>
  )
}
