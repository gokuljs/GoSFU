import { useRef, useEffect, useState } from "react"
import { Orb, type AgentState } from "@/components/ui/orb"
import { ShimmeringText } from "@/components/ui/shimmering-text"
import { useStreamAudioLevel } from "@/hooks/use-stream-audio-level"

type TileType = "human" | "agent"

interface VideoTileProps {
  stream: MediaStream | null
  muted?: boolean
  label: string
  type?: TileType
}

export function VideoTile({
  stream,
  muted = false,
  label,
  type = "human",
}: VideoTileProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const audioRef = useRef<HTMLAudioElement>(null)
  const { levelRef, isSpeaking, hasAudio } = useStreamAudioLevel(
    type === "agent" ? stream : null
  )

  useEffect(() => {
    if (videoRef.current && stream) {
      videoRef.current.srcObject = stream
    }
  }, [stream])

  useEffect(() => {
    if (type !== "agent" || !audioRef.current || !stream) return

    const el = audioRef.current
    const play = () => {
      el.srcObject = stream
      void el.play().catch(() => {})
    }

    play()
    stream.addEventListener("addtrack", play)
    return () => {
      stream.removeEventListener("addtrack", play)
      el.pause()
      el.srcObject = null
    }
  }, [stream, type])

  const agentState: AgentState =
    type === "agent"
      ? isSpeaking
        ? "talking"
        : hasAudio
          ? "listening"
          : null
      : null

  const stateLabel =
    agentState === "listening"
      ? "Listening..."
      : agentState === "talking"
        ? "Speaking..."
        : stream
          ? "Connecting..."
          : "Idle"

  const [videoEnabled, setVideoEnabled] = useState(true)

  useEffect(() => {
    if (!stream) return
    const checkVideo = () => {
      const videoTrack = stream.getVideoTracks()[0]
      setVideoEnabled(!!videoTrack && videoTrack.enabled)
    }
    checkVideo()
    const interval = setInterval(checkVideo, 300)
    return () => clearInterval(interval)
  }, [stream])

  const hasVideo = stream && stream.getVideoTracks().length > 0 && videoEnabled

  return (
    <div className="relative flex aspect-video w-full items-center justify-center overflow-hidden rounded-xl bg-black">
      {type === "agent" && (
        <audio ref={audioRef} autoPlay playsInline className="hidden" />
      )}

      {type === "human" && hasVideo ? (
        <video
          ref={videoRef}
          autoPlay
          playsInline
          muted={muted}
          className="h-full w-full object-cover"
        />
      ) : type === "human" ? (
        <div className="flex items-center justify-center">
          <img
            src="/avatar.png"
            alt="You"
            className="h-32 w-32 rounded-full object-cover opacity-90 sm:h-36 sm:w-36"
          />
        </div>
      ) : (
        <div className="flex flex-col items-center gap-5">
          <div className="h-36 w-36 overflow-hidden rounded-full sm:h-40 sm:w-40 md:h-44 md:w-44">
            <Orb
              colors={["#6366f1", "#a78bfa"]}
              agentState={agentState}
              volumeMode="manual"
              outputVolumeRef={levelRef}
              seed={42}
            />
          </div>
          <ShimmeringText
            text={stateLabel}
            className="text-sm font-medium"
            color="rgba(255, 255, 255, 0.4)"
            shimmerColor="rgba(255, 255, 255, 0.9)"
            duration={2}
            repeat={true}
            repeatDelay={0.5}
            spread={2}
          />
        </div>
      )}

      <div className="absolute bottom-3 left-3 rounded-md bg-black/50 px-2 py-1 text-[11px] font-medium text-white/50 backdrop-blur-sm">
        {label}
      </div>
    </div>
  )
}
