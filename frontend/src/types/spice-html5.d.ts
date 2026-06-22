declare module 'spice-html5-react' {
  export type SpiceStatus = 'disconnected' | 'connecting' | 'connected' | 'error'

  export interface SpiceResolution {
    width: number
    height: number
  }

  export interface SpiceConfig {
    uri: string
    password?: string
    screenId?: string
    dumpId?: string
    messageId?: string
  }

  export interface UseSpiceCallbacks {
    onConnect?: () => void
    onDisconnect?: () => void
    onError?: (error: Error) => void
    onResize?: (width: number, height: number) => void
  }

  export interface UseSpiceOptions {
    canvasRef?: React.RefObject<HTMLElement | null>
    callbacks?: UseSpiceCallbacks
  }

  export interface SpiceControls {
    connect: (config: SpiceConfig) => void
    disconnect: () => void
    sendKeyDown: (scancode: number) => void
    sendKeyUp: (scancode: number) => void
    sendMouseMove: (x: number, y: number) => void
    sendMouseButton: (button: number, pressed: boolean) => void
    sendClipboard: (text: string) => void
    setResolution: (width: number, height: number) => void
    sendCtrlAltDel: () => void
    getConnection: () => any | null
  }

  export interface UseSpiceReturn {
    status: SpiceStatus
    error: Error | null
    resolution: SpiceResolution | null
    surfaces: number
    controls: SpiceControls
  }

  export function useSpice(options?: UseSpiceOptions): UseSpiceReturn
  export function SpiceDisplay(props: any): JSX.Element
}
