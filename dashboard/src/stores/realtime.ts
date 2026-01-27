import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import type {
  ConnectionStatus,
  RealtimeEvent,
  EventHandler,
  RealtimeSubscription,
} from '@/types/realtime'
import { SSE_ENDPOINTS } from '@/types/realtime'
import { useUIStore } from './ui'

interface SSEConnection {
  endpoint: string
  eventSource: EventSource | null
  status: ConnectionStatus
  reconnectAttempts: number
  lastError: string | null
}

const MAX_RECONNECT_ATTEMPTS = 5
const BASE_RECONNECT_DELAY = 1000
const MAX_RECENT_EVENTS = 50

export const useRealtimeStore = defineStore('realtime', () => {
  // Connections state
  const connections = ref<Map<string, SSEConnection>>(new Map())

  // Event subscriptions
  const subscriptions = ref<RealtimeSubscription[]>([])
  let subscriptionCounter = 0

  // Recent events for debugging
  const recentEvents = ref<RealtimeEvent[]>([])

  // Global enabled state
  const enabled = ref(false)

  // Reconnect timers
  const reconnectTimers = new Map<string, ReturnType<typeof setTimeout>>()

  // Getters
  const isConnected = computed(() => {
    for (const conn of connections.value.values()) {
      if (conn.status === 'connected') return true
    }
    return false
  })

  const isConnecting = computed(() => {
    for (const conn of connections.value.values()) {
      if (conn.status === 'connecting') return true
    }
    return false
  })

  const hasError = computed(() => {
    for (const conn of connections.value.values()) {
      if (conn.status === 'error') return true
    }
    return false
  })

  const connectionStatuses = computed(() => {
    const statuses: Record<string, ConnectionStatus> = {}
    for (const [name, conn] of connections.value.entries()) {
      statuses[name] = conn.status
    }
    return statuses
  })

  const overallStatus = computed<ConnectionStatus>(() => {
    if (!enabled.value) return 'disconnected'
    if (hasError.value) return 'error'
    if (isConnecting.value) return 'connecting'
    if (isConnected.value) return 'connected'
    return 'disconnected'
  })

  // Actions
  function connect(name: string) {
    const endpoint = SSE_ENDPOINTS[name]
    if (!endpoint) {
      console.error(`Unknown SSE endpoint: ${name}`)
      return
    }

    // Check if already connected
    const existing = connections.value.get(name)
    if (existing?.eventSource) {
      existing.eventSource.close()
    }

    const conn: SSEConnection = {
      endpoint: endpoint.url,
      eventSource: null,
      status: 'connecting',
      reconnectAttempts: 0,
      lastError: null,
    }

    connections.value.set(name, conn)

    try {
      const eventSource = new EventSource(endpoint.url)

      eventSource.onopen = () => {
        const connection = connections.value.get(name)
        if (connection) {
          connection.status = 'connected'
          connection.reconnectAttempts = 0
          connection.lastError = null
        }
      }

      eventSource.onmessage = (event) => {
        handleEvent(name, event)
      }

      eventSource.onerror = () => {
        handleConnectionError(name)
      }

      conn.eventSource = eventSource
      connections.value.set(name, conn)
    } catch (err) {
      conn.status = 'error'
      conn.lastError = err instanceof Error ? err.message : 'Connection failed'
      connections.value.set(name, conn)
    }
  }

  function disconnect(name: string) {
    const conn = connections.value.get(name)
    if (!conn) return

    // Clear reconnect timer
    const timer = reconnectTimers.get(name)
    if (timer) {
      clearTimeout(timer)
      reconnectTimers.delete(name)
    }

    // Close event source
    if (conn.eventSource) {
      conn.eventSource.close()
    }

    conn.eventSource = null
    conn.status = 'disconnected'
    conn.reconnectAttempts = 0
    connections.value.set(name, conn)
  }

  function handleConnectionError(name: string) {
    const conn = connections.value.get(name)
    if (!conn) return

    conn.status = 'error'
    conn.lastError = 'Connection lost'

    // Auto-reconnect with exponential backoff
    if (enabled.value && conn.reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
      const delay = BASE_RECONNECT_DELAY * Math.pow(2, conn.reconnectAttempts)
      conn.reconnectAttempts++

      const timer = setTimeout(() => {
        reconnectTimers.delete(name)
        connect(name)
      }, delay)

      reconnectTimers.set(name, timer)
    } else {
      conn.status = 'disconnected'
    }

    connections.value.set(name, conn)
  }

  function handleEvent(source: string, event: MessageEvent) {
    try {
      const data = JSON.parse(event.data) as RealtimeEvent

      // Store in recent events
      recentEvents.value.unshift(data)
      if (recentEvents.value.length > MAX_RECENT_EVENTS) {
        recentEvents.value.pop()
      }

      // Dispatch to subscribers
      for (const sub of subscriptions.value) {
        if (sub.eventType === '*' || sub.eventType === data.type) {
          try {
            sub.handler(data)
          } catch (err) {
            console.error(`Error in event handler for ${sub.eventType}:`, err)
          }
        }
      }
    } catch (err) {
      console.error(`Failed to parse SSE event from ${source}:`, err)
    }
  }

  function subscribe<T extends RealtimeEvent = RealtimeEvent>(
    eventType: string,
    handler: EventHandler<T>
  ): () => void {
    const id = `sub_${++subscriptionCounter}`
    const subscription: RealtimeSubscription = {
      id,
      eventType,
      handler: handler as EventHandler,
    }

    subscriptions.value.push(subscription)

    // Return unsubscribe function
    return () => {
      const index = subscriptions.value.findIndex((s) => s.id === id)
      if (index !== -1) {
        subscriptions.value.splice(index, 1)
      }
    }
  }

  function connectAll() {
    enabled.value = true
    for (const name of Object.keys(SSE_ENDPOINTS)) {
      connect(name)
    }
  }

  function disconnectAll() {
    enabled.value = false
    for (const name of connections.value.keys()) {
      disconnect(name)
    }
  }

  function clearRecentEvents() {
    recentEvents.value = []
  }

  // Watch for feature flag changes
  function init() {
    const uiStore = useUIStore()

    // Check if realtime is enabled via feature flag
    if (uiStore.isFeatureEnabled('realtimeUpdates')) {
      connectAll()
    }

    // Watch for feature flag changes
    watch(
      () => uiStore.isFeatureEnabled('realtimeUpdates'),
      (newVal) => {
        if (newVal) {
          connectAll()
        } else {
          disconnectAll()
        }
      }
    )
  }

  function $reset() {
    disconnectAll()
    subscriptions.value = []
    recentEvents.value = []
    connections.value.clear()
  }

  return {
    // State
    connections,
    subscriptions,
    recentEvents,
    enabled,

    // Getters
    isConnected,
    isConnecting,
    hasError,
    connectionStatuses,
    overallStatus,

    // Actions
    connect,
    disconnect,
    connectAll,
    disconnectAll,
    subscribe,
    clearRecentEvents,
    init,

    $reset,
  }
})
