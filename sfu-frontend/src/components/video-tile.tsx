import { useRef, useEffect } from "react"
import { User } from "@phosphor-icons/react"

interface VideoTileProps {
  stream: MediaStream | null
  muted?: boolean
  label: string
  isAvatar?: boolean
}

export function VideoTile({
  stream,
  muted = false,
  label,
  isAvatar = false,
}: VideoTileProps) {
  const videoRef = useRef<HTMLVideoElement>(null)

  useEffect(() => {
    if (videoRef.current && stream) {
      videoRef.current.srcObject = stream
    }
  }, [stream])

  return (
    <div className="relative flex aspect-video w-full items-center justify-center overflow-hidden rounded-xl border border-white/5 bg-[#111111]">
      {stream && !isAvatar ? (
        <video
          ref={videoRef}
          autoPlay
          playsInline
          muted={muted}
          className="h-full w-full object-cover"
        />
      ) : (
        <div className="flex flex-col items-center gap-3">
          <div className="flex h-16 w-16 items-center justify-center rounded-full bg-white/5 sm:h-20 sm:w-20">
            <User className="h-8 w-8 text-white/30 sm:h-10 sm:w-10" />
          </div>
          <span className="text-xs text-white/30">Waiting for peer...</span>
        </div>
      )}

      <div className="absolute bottom-3 left-3 rounded-md bg-black/60 px-2 py-1 text-[11px] font-medium text-white/70 backdrop-blur-sm">
        {label}
      </div>
    </div>
  )
}
