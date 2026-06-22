declare module '@novnc/novnc/core/rfb' {
  export interface RFBOptions {
    wsProtocols?: string[]
    shared?: boolean
    credentials?: (credType: string) => { password: string }
    repeaterID?: string
  }

  export default class RFB {
    constructor(target: HTMLElement, url: string, options?: RFBOptions)

    disconnect(): void
    sendCredentials(credentials: { password: string }): void
    sendKey(keysym: number, code: string, down?: boolean): void
    focus(): void
    blur(): void
    clipboardPasteFrom(text: string): void
    getImageData(): string | null

    scaleViewport: boolean
    resizeSession: boolean
    showDotCursor: boolean
    viewportScale: number
    clientWidth: number
    clientHeight: number

    addEventListener(type: string, listener: (e: CustomEvent) => void): void
    removeEventListener(type: string, listener: (e: CustomEvent) => void): void
  }
}
