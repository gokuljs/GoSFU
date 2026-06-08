import { createContext, useCallback, useContext, useEffect, type ReactNode } from "react"
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
  useNavigate,
  useParams,
} from "react-router-dom"
import { LandingPage } from "@/pages/landing"
import { RoomPage } from "@/pages/room"
import { SFU_URL, useWebRTC } from "@/hooks/use-webrtc"
import { useRoomStream } from "@/hooks/use-room-stream"

type WebRTCContextValue = ReturnType<typeof useWebRTC>

const WebRTCContext = createContext<WebRTCContextValue | null>(null)

function WebRTCProvider({ children }: { children: ReactNode }) {
  const value = useWebRTC()
  return <WebRTCContext.Provider value={value}>{children}</WebRTCContext.Provider>
}

function useWebRTCContext() {
  const value = useContext(WebRTCContext)
  if (!value) {
    throw new Error("useWebRTCContext must be used within WebRTCProvider")
  }
  return value
}

function LandingRoute() {
  const navigate = useNavigate()
  const { createRoom } = useWebRTCContext()

  const handleConnect = useCallback(async () => {
    const roomId = await createRoom()
    navigate(`/room/${roomId}`)
  }, [createRoom, navigate])

  return <LandingPage onConnect={handleConnect} />
}

function RoomRoute() {
  const { roomId: routeRoomId } = useParams()
  const navigate = useNavigate()
  const {
    localStream,
    remoteStream,
    roomId,
    participantId,
    connectionState,
    peerConnectionState,
    iceConnectionState,
    selectedDevices,
    connect,
    disconnect,
    toggleMic,
    toggleCamera,
    isMicOn,
    isCameraOn,
  } = useWebRTCContext()
  const activeRoomId = roomId ?? routeRoomId ?? null
  const stream = useRoomStream(activeRoomId, SFU_URL)
  const { addLocalEvent } = stream

  useEffect(() => {
    if (!routeRoomId) return
    void connect(routeRoomId)
    return () => {
      disconnect()
    }
  }, [routeRoomId, connect, disconnect])

  const handleDisconnect = useCallback(() => {
    disconnect()
    navigate("/")
  }, [disconnect, navigate])

  useEffect(() => {
    addLocalEvent("client.connection.state", "Client connection state", {
      state: connectionState,
      room_id: routeRoomId,
    })
  }, [addLocalEvent, routeRoomId, connectionState])

  useEffect(() => {
    addLocalEvent(
      "client.peer_connection.state",
      "Client peer connection state",
      { state: peerConnectionState, room_id: routeRoomId }
    )
  }, [addLocalEvent, routeRoomId, peerConnectionState])

  useEffect(() => {
    addLocalEvent("client.ice.state", "Client ICE state", {
      state: iceConnectionState,
      room_id: routeRoomId,
    })
  }, [addLocalEvent, routeRoomId, iceConnectionState])

  useEffect(() => {
    addLocalEvent("client.media.mic", "Microphone toggled", {
      enabled: isMicOn,
      room_id: routeRoomId,
    })
  }, [addLocalEvent, routeRoomId, isMicOn])

  useEffect(() => {
    addLocalEvent("client.media.camera", "Camera toggled", {
      enabled: isCameraOn,
      room_id: routeRoomId,
    })
  }, [addLocalEvent, routeRoomId, isCameraOn])

  if (!routeRoomId) {
    return <Navigate to="/" replace />
  }

  return (
    <RoomPage
      localStream={localStream}
      remoteStream={remoteStream}
      roomId={routeRoomId}
      participantId={participantId}
      connectionState={connectionState}
      peerConnectionState={peerConnectionState}
      streamStatus={stream.status}
      selectedDevices={selectedDevices}
      debugEvents={stream.debugEvents}
      transcript={stream.transcript}
      metrics={stream.metrics}
      latestByStage={stream.latestByStage}
      onClearEvents={stream.clearEvents}
      isMicOn={isMicOn}
      isCameraOn={isCameraOn}
      onToggleMic={toggleMic}
      onToggleCamera={toggleCamera}
      onDisconnect={handleDisconnect}
    />
  )
}

export function App() {
  return (
    <BrowserRouter>
      <WebRTCProvider>
        <Routes>
          <Route path="/" element={<LandingRoute />} />
          <Route path="/room/:roomId" element={<RoomRoute />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </WebRTCProvider>
    </BrowserRouter>
  )
}

export default App
