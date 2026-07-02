import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react"
import {
  BrowserRouter,
  Navigate,
  Route,
  Routes,
  useNavigate,
  useParams,
} from "react-router-dom"
import { Toaster, toast } from "sonner"
import { LandingPage } from "@/pages/landing"
import { RoomPage } from "@/pages/room"
import { SFU_URL, useWebRTC } from "@/hooks/use-webrtc"
import { useRoomStream } from "@/hooks/use-room-stream"
import { useSessionTimer } from "@/hooks/use-session-timer"
import { DEFAULT_SYSTEM_PROMPT } from "@/lib/agent-defaults"

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
    stopSession,
    deleteRoom,
    toggleMic,
    toggleCamera,
    isMicOn,
    isCameraOn,
  } = useWebRTCContext()
  const activeRoomId = roomId ?? routeRoomId ?? null
  const stream = useRoomStream(activeRoomId, SFU_URL)
  const { addLocalEvent } = stream
  const [systemPrompt, setSystemPrompt] = useState(DEFAULT_SYSTEM_PROMPT)
  const expiredHandledRef = useRef(false)
  const sessionActive = connectionState === "connected"
  const sessionTimer = useSessionTimer(sessionActive)

  useEffect(() => {
    expiredHandledRef.current = false
  }, [routeRoomId])

  useEffect(() => {
    return () => {
      disconnect()
    }
  }, [routeRoomId, disconnect])

  useEffect(() => {
    if (connectionState !== "connected" && connectionState !== "connecting") {
      return
    }
    const onLeave = () => {
      if (!routeRoomId) return
      navigator.sendBeacon(`${SFU_URL}/room/${routeRoomId}/leave`, "")
    }
    window.addEventListener("pagehide", onLeave)
    return () => window.removeEventListener("pagehide", onLeave)
  }, [connectionState, routeRoomId])

  useEffect(() => {
    const expired = stream.debugEvents.some(
      (event) => event.type === "session.room.expired"
    )
    if (!expired || expiredHandledRef.current) return

    expiredHandledRef.current = true
    toast.error("Session ended", {
      id: `session-expired-${routeRoomId ?? "room"}`,
      description: "This room reached the maximum session duration.",
      duration: 8000,
    })
    disconnect()
    navigate("/")
  }, [disconnect, navigate, routeRoomId, stream.debugEvents])

  const handleStartSession = useCallback(() => {
    if (!routeRoomId) return
    void connect(routeRoomId, systemPrompt)
  }, [connect, routeRoomId, systemPrompt])

  const handleStopSession = useCallback(() => {
    if (!routeRoomId) return
    void stopSession(routeRoomId)
  }, [routeRoomId, stopSession])

  const handleDisconnect = useCallback(async () => {
    disconnect()
    navigate("/")

    if (routeRoomId) {
      try {
        await deleteRoom(routeRoomId)
      } catch (err) {
        console.warn("Room cleanup failed:", err)
      }
    }
  }, [deleteRoom, disconnect, navigate, routeRoomId])

  const canEditPrompt =
    connectionState === "idle" || connectionState === "failed"

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
      systemPrompt={systemPrompt}
      onSystemPromptChange={setSystemPrompt}
      onStartSession={handleStartSession}
      onStopSession={handleStopSession}
      canEditPrompt={canEditPrompt}
      sessionTimerLabel={sessionActive ? sessionTimer.label : null}
      sessionTimerWarning={sessionTimer.isWarning}
    />
  )
}

export function App() {
  return (
    <>
      <BrowserRouter>
        <WebRTCProvider>
          <Routes>
            <Route path="/" element={<LandingRoute />} />
            <Route path="/room/:roomId" element={<RoomRoute />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </WebRTCProvider>
      </BrowserRouter>
      <Toaster
        theme="dark"
        position="top-right"
        toastOptions={{
          classNames: {
            toast: "border border-[#1a1a1a] bg-[#0a0a0a] text-white",
            title: "text-white",
            description: "text-white/50",
            error: "border-[#ff4444]/30",
          },
        }}
      />
    </>
  )
}

export default App
