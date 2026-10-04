import { useAppStore } from '@/stores/app'

export function Footer() {
  const footerText = useAppStore((s) => s.getThemeConfig('footerText'))
  const siteName = useAppStore((s) => s.getThemeConfig('siteName'))

  return (
    <footer className="border-t border-border py-6 mt-auto">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 text-center text-sm text-muted-foreground">
        {footerText || `${siteName} - Powered by Tsukiyo`}
      </div>
    </footer>
  )
}
