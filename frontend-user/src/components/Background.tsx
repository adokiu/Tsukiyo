import { useAppStore } from '@/stores/app'

export function Background() {
  const getThemeConfig = useAppStore((s) => s.getThemeConfig)
  const isDark = useAppStore((s) => s.getIsDark())
  const enabled = getThemeConfig('backgroundEnabled')
  const bgType = getThemeConfig('backgroundType')
  const bgUrl = getThemeConfig(isDark ? 'darkBackgroundUrl' : 'lightBackgroundUrl')
  const blur = getThemeConfig('backgroundBlur')
  const overlay = getThemeConfig('backgroundOverlay')

  if (!enabled || !bgUrl) {
    return null
  }

  return (
    <div className="fixed inset-0 -z-10 overflow-hidden">
      {bgType === 'video' ? (
        <video
          src={bgUrl}
          autoPlay
          loop
          muted
          className="w-full h-full object-cover"
          style={{ filter: blur > 0 ? `blur(${blur}px)` : undefined }}
        />
      ) : (
        <div
          className="w-full h-full bg-cover bg-center bg-no-repeat"
          style={{
            backgroundImage: `url(${bgUrl})`,
            filter: blur > 0 ? `blur(${blur}px)` : undefined,
          }}
        />
      )}
      {overlay > 0 && (
        <div
          className="absolute inset-0"
          style={{
            backgroundColor: isDark ? `rgba(0,0,0,${overlay / 100})` : `rgba(255,255,255,${overlay / 100})`,
          }}
        />
      )}
    </div>
  )
}
