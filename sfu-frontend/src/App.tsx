import { useState, useCallback } from "react"
import { LandingPage } from "@/pages/landing"
import { RoomPage } from "@/pages/room"
import { useWebRTC } from "@/hooks/use-webrtc"

type View = "landing" | "room"

export function App() {
  const [view, setView] = useState<View>("landing")
  const {
    localStream,
    remoteStream,
    connectionState,
    connect,
    disconnect,
    toggleMic,
    toggleCamera,
    isMicOn,
    isCameraOn,
  } = useWebRTC()

  const handleConnect = useCallback(async () => {
    setView("room")
    await connect()
  }, [connect])

  const handleDisconnect = useCallback(() => {
    disconnect()
    setView("landing")
  }, [disconnect])

  return (
    <div className="relative min-h-svh overflow-hidden bg-[#0a0a0a]">
      <div
        className={`absolute inset-0 transition-opacity duration-500 ${
          view === "landing"
            ? "pointer-events-auto opacity-100"
            : "pointer-events-none opacity-0"
        }`}
      >
        <LandingPage onConnect={handleConnect} />
      </div>

      <div
        className={`absolute inset-0 transition-opacity duration-500 ${
          view === "room"
            ? "pointer-events-auto opacity-100"
            : "pointer-events-none opacity-0"
        }`}
      >
        <RoomPage
          localStream={localStream}
          remoteStream={remoteStream}
          connectionState={connectionState}
          isMicOn={isMicOn}
          isCameraOn={isCameraOn}
          onToggleMic={toggleMic}
          onToggleCamera={toggleCamera}
          onDisconnect={handleDisconnect}
        />
      </div>
    </div>
  )
}

export default App
