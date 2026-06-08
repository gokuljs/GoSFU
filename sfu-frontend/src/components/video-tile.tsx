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
  compact?: boolean
}

export function VideoTile({
  stream,
  muted = false,
  label,
  type = "human",
  compact = false,
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

  const [videoEnabled, setVideoEnabled] = useState(false)

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
    <div className="relative flex h-full w-full items-center justify-center overflow-hidden border border-[#1a1a1a] bg-[#050505]">
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
            className={`${compact ? "h-20 w-20" : "h-32 w-32 sm:h-36 sm:w-36"} rounded-full object-cover opacity-70`}
          />
        </div>
      ) : (
        <div className={`flex flex-col items-center ${compact ? "gap-2" : "gap-5"}`}>
          <div
            className={`${compact ? "h-20 w-20" : "h-36 w-36 sm:h-40 sm:w-40 md:h-44 md:w-44"} overflow-hidden rounded-full`}
          >
            <Orb
              colors={["#00d4aa", "#00b894"]}
              agentState={agentState}
              volumeMode="manual"
              outputVolumeRef={levelRef}
              seed={42}
            />
          </div>
          <ShimmeringText
            text={stateLabel}
            className={`${compact ? "text-[9px]" : "text-[10px]"} font-medium tracking-wider uppercase`}
            color="rgba(255, 255, 255, 0.3)"
            shimmerColor="rgba(0, 212, 170, 0.8)"
            duration={2}
            repeat={true}
            repeatDelay={0.5}
            spread={2}
          />
        </div>
      )}

      <div className="absolute bottom-2 left-2 border border-[#1a1a1a] bg-[#050505]/80 px-2 py-0.5 text-[9px] font-medium tracking-wider text-[#00d4aa]/60 uppercase">
        {label}
      </div>
    </div>
  )
}
