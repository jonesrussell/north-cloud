import { ref, onUnmounted, computed } from 'vue'

export type ConnectionStatus = 'connected' | 'connecting' | 'disconnected' | 'error'

export interface RealtimeOptions {
  /** Auto-reconnect on connection loss (default: true) */
  autoReconnect?: boolean
  /** Maximum reconnect attempts (default: 5) */
  maxReconnectAttempts?: number
  /** Base delay between reconnect attempts in ms (default: 1000) */
  reconnectDelay?: number
}

/**
 * SSE connection composable for real-time updates
 *
 * Note: This is a Phase 2 feature. Backend SSE endpoints must be implemented first.
 * Currently provides the interface for when SSE is available.
 *
 * @param endpoint - SSE endpoint URL
 * @param options - Connection options
 */
export function useSSE<T = unknown>(endpoint: string, options: RealtimeOptions = {}) {
  const {
    autoReconnect = true,
    maxReconnectAttempts = 5,
    reconnectDelay = 1000,
  } = options

  const status = ref<ConnectionStatus>('disconnected')
  const data = ref<T | null>(null)
  const error = ref<string | null>(null)
  const reconnectAttempts = ref(0)

  let eventSource: EventSource | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  const isConnected = computed(() => status.value === 'connected')

  function connect() {
    if (eventSource) {
      disconnect()
    }

    status.value = 'connecting'
    error.value = null

    try {
      eventSource = new EventSource(endpoint)

      eventSource.onopen = () => {
        status.value = 'connected'
        reconnectAttempts.value = 0
      }

      eventSource.onmessage = (event) => {
        try {
          data.value = JSON.parse(event.data)
        } catch {
          data.value = event.data as T
        }
      }

      eventSource.onerror = () => {
        status.value = 'error'
        error.value = 'Connection lost'

        // Auto-reconnect with exponential backoff
        if (autoReconnect && reconnectAttempts.value < maxReconnectAttempts) {
          const delay = reconnectDelay * Math.pow(2, reconnectAttempts.value)
          reconnectAttempts.value++

          reconnectTimer = setTimeout(() => {
            connect()
          }, delay)
        } else {
          status.value = 'disconnected'
        }
      }
    } catch (err) {
      status.value = 'error'
      error.value = err instanceof Error ? err.message : 'Failed to connect'
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }

    if (eventSource) {
      eventSource.close()
      eventSource = null
    }

    status.value = 'disconnected'
    reconnectAttempts.value = 0
  }

  function subscribe(eventType: string, handler: (data: T) => void) {
    if (!eventSource) return

    eventSource.addEventListener(eventType, (event) => {
      try {
        const parsed = JSON.parse((event as MessageEvent).data)
        handler(parsed)
      } catch {
        handler((event as MessageEvent).data as T)
      }
    })
  }

  onUnmounted(() => {
    disconnect()
  })

  return {
    status,
    data,
    error,
    isConnected,
    reconnectAttempts,
    connect,
    disconnect,
    subscribe,
  }
}

/**
 * WebSocket connection composable for bidirectional communication
 *
 * Note: This is a Phase 2 feature for log streaming.
 *
 * @param endpoint - WebSocket endpoint URL
 * @param options - Connection options
 */
export function useWebSocket<T = unknown>(endpoint: string, options: RealtimeOptions = {}) {
  const {
    autoReconnect = true,
    maxReconnectAttempts = 5,
    reconnectDelay = 1000,
  } = options

  const status = ref<ConnectionStatus>('disconnected')
  const data = ref<T | null>(null)
  const error = ref<string | null>(null)
  const reconnectAttempts = ref(0)

  let socket: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null

  const isConnected = computed(() => status.value === 'connected')

  function connect() {
    if (socket) {
      disconnect()
    }

    status.value = 'connecting'
    error.value = null

    try {
      socket = new WebSocket(endpoint)

      socket.onopen = () => {
        status.value = 'connected'
        reconnectAttempts.value = 0
      }

      socket.onmessage = (event) => {
        try {
          data.value = JSON.parse(event.data)
        } catch {
          data.value = event.data as T
        }
      }

      socket.onerror = () => {
        status.value = 'error'
        error.value = 'Connection error'
      }

      socket.onclose = () => {
        status.value = 'disconnected'

        // Auto-reconnect with exponential backoff
        if (autoReconnect && reconnectAttempts.value < maxReconnectAttempts) {
          const delay = reconnectDelay * Math.pow(2, reconnectAttempts.value)
          reconnectAttempts.value++

          reconnectTimer = setTimeout(() => {
            connect()
          }, delay)
        }
      }
    } catch (err) {
      status.value = 'error'
      error.value = err instanceof Error ? err.message : 'Failed to connect'
    }
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }

    if (socket) {
      socket.close()
      socket = null
    }

    status.value = 'disconnected'
    reconnectAttempts.value = 0
  }

  function send(message: unknown) {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket is not connected')
    }

    const payload = typeof message === 'string' ? message : JSON.stringify(message)
    socket.send(payload)
  }

  onUnmounted(() => {
    disconnect()
  })

  return {
    status,
    data,
    error,
    isConnected,
    reconnectAttempts,
    connect,
    disconnect,
    send,
  }
}
