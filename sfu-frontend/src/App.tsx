import { useState, useCallback, useEffect } from "react"
import { LandingPage } from "@/pages/landing"
import { RoomPage } from "@/pages/room"
import { SFU_URL, useWebRTC } from "@/hooks/use-webrtc"
import { useSessionDebug } from "@/hooks/use-session-debug"

type View = "landing" | "room"

export function App() {
  const [view, setView] = useState<View>("landing")
  const {
    localStream,
    remoteStream,
    roomId,
    participantId,
    connectionState,
    peerConnectionState,
    iceConnectionState,
    devices,
    selectedDevices,
    connect,
    disconnect,
    toggleMic,
    toggleCamera,
    isMicOn,
    isCameraOn,
  } = useWebRTC()
  const debug = useSessionDebug(roomId, SFU_URL)
  const { addLocalEvent } = debug

  const handleConnect = useCallback(async () => {
    setView("room")
    await connect()
  }, [connect])

  const handleDisconnect = useCallback(() => {
    disconnect()
    setView("landing")
  }, [disconnect])

  useEffect(() => {
    if (view !== "room") return
    addLocalEvent("client.connection.state", "Client connection state", {
      state: connectionState,
    })
  }, [addLocalEvent, connectionState, view])

  useEffect(() => {
    if (view !== "room") return
    addLocalEvent(
      "client.peer_connection.state",
      "Client peer connection state",
      { state: peerConnectionState }
    )
  }, [addLocalEvent, peerConnectionState, view])

  useEffect(() => {
    if (view !== "room") return
    addLocalEvent("client.ice.state", "Client ICE state", {
      state: iceConnectionState,
    })
  }, [addLocalEvent, iceConnectionState, view])

  useEffect(() => {
    if (view !== "room") return
    addLocalEvent("client.media.mic", "Microphone toggled", {
      enabled: isMicOn,
    })
  }, [addLocalEvent, isMicOn, view])

  useEffect(() => {
    if (view !== "room") return
    addLocalEvent("client.media.camera", "Camera toggled", {
      enabled: isCameraOn,
    })
  }, [addLocalEvent, isCameraOn, view])

  return (
    <div className="relative min-h-svh overflow-hidden bg-black">
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
          roomId={roomId}
          participantId={participantId}
          connectionState={connectionState}
          peerConnectionState={peerConnectionState}
          iceConnectionState={iceConnectionState}
          debugStatus={debug.status}
          devices={devices}
          selectedDevices={selectedDevices}
          debugEvents={debug.events}
          transcript={debug.transcript}
          onClearEvents={debug.clearEvents}
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
