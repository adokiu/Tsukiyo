interface OSConfig {
  name: string
  image: string
  keywords: string[]
}

const osConfigs: OSConfig[] = [
  { name: 'AlmaLinux', image: '/images/logo/os-alma.svg', keywords: ['alma', 'almalinux'] },
  { name: 'Alpine Linux', image: '/images/logo/os-alpine.webp', keywords: ['alpine'] },
  { name: 'Armbian', image: '/images/logo/os-armbian.svg', keywords: ['armbian'] },
  { name: 'CentOS', image: '/images/logo/os-centos.svg', keywords: ['centos', 'cent os'] },
  { name: 'Debian', image: '/images/logo/os-debian.svg', keywords: ['debian', 'deb'] },
  { name: 'FreeBSD', image: '/images/logo/os-freebsd.svg', keywords: ['freebsd', 'bsd'] },
  { name: 'Ubuntu', image: '/images/logo/os-ubuntu.svg', keywords: ['ubuntu', 'elementary'] },
  { name: 'Windows', image: '/images/logo/os-windows.svg', keywords: ['windows', 'win', 'microsoft'] },
  { name: 'Arch Linux', image: '/images/logo/os-arch.svg', keywords: ['arch', 'archlinux'] },
  { name: 'Kali Linux', image: '/images/logo/os-kail.svg', keywords: ['kail', 'kali'] },
  { name: 'iStoreOS', image: '/images/logo/os-istore.png', keywords: ['istore'] },
  { name: 'OpenWrt', image: '/images/logo/os-openwrt.svg', keywords: ['openwrt', 'open wrt', 'qwrt'] },
  { name: 'ImmortalWrt', image: '/images/logo/os-openwrt.svg', keywords: ['immortalwrt', 'immortal'] },
  { name: 'NixOS', image: '/images/logo/os-nix.svg', keywords: ['nixos', 'nix'] },
  { name: 'Rocky Linux', image: '/images/logo/os-rocky.svg', keywords: ['rocky'] },
  { name: 'Fedora', image: '/images/logo/os-fedora.svg', keywords: ['fedora'] },
  { name: 'openSUSE', image: '/images/logo/os-openSUSE.svg', keywords: ['opensuse', 'suse'] },
  { name: 'Gentoo', image: '/images/logo/os-gentoo.svg', keywords: ['gentoo'] },
  { name: 'Red Hat', image: '/images/logo/os-redhat.svg', keywords: ['redhat', 'rhel', 'red hat'] },
  { name: 'Linux Mint', image: '/images/logo/os-mint.svg', keywords: ['mint'] },
  { name: 'Manjaro', image: '/images/logo/os-manjaro-.svg', keywords: ['manjaro'] },
  { name: 'Synology DSM', image: '/images/logo/os-synology.ico', keywords: ['synology', 'dsm'] },
  { name: 'fnOS', image: '/images/logo/os-fnos.ico', keywords: ['fnos', 'fnnas'] },
  { name: 'Proxmox VE', image: '/images/logo/os-proxmox.ico', keywords: ['proxmox'] },
  { name: 'macOS', image: '/images/logo/os-macos.svg', keywords: ['macos'] },
  { name: 'QTS', image: '/images/logo/os-qnap.svg', keywords: ['qts', 'quts'] },
  { name: 'Astra Linux', image: '/images/logo/os-astar.png', keywords: ['astra'] },
  { name: 'Orange Pi', image: '/images/logo/os-orange-pi.svg', keywords: ['orange pi', 'orangepi'] },
  { name: 'Huawei', image: '/images/logo/os-huawei.svg', keywords: ['huawei', 'euleros'] },
  { name: 'openEuler', image: '/images/logo/os-openEuler.svg', keywords: ['openeuler', 'euler'] },
  { name: 'Aliyun', image: '/images/logo/alibabacloud-color.svg', keywords: ['aliyun', 'alibaba'] },
  { name: 'OpenCloudOS', image: '/images/logo/os-OpenCloudOS.png', keywords: ['opencloud'] },
  { name: 'Unraid', image: '/images/logo/os-unraid.svg', keywords: ['unraid'] },
  { name: 'Oracle Linux', image: '/images/logo/os-oracle.svg', keywords: ['oracle'] },
]

const defaultOSConfig: OSConfig = {
  name: 'Unknown',
  image: '/images/logo/linux.svg',
  keywords: ['unknown'],
}

function findOSConfig(osString: string): OSConfig {
  if (!osString) return defaultOSConfig
  const normalized = osString.toLowerCase().trim()
  for (const config of osConfigs) {
    for (const keyword of config.keywords) {
      if (normalized.includes(keyword)) return config
    }
  }
  return defaultOSConfig
}

export function getOSImage(osString: string): string {
  return findOSConfig(osString).image
}

export function getOSDisplayName(osString: string): string {
  const config = findOSConfig(osString)
  if (config !== defaultOSConfig) return config.name
  if (!osString) return 'Unknown'
  const parts = osString.trim().split(/[\s/]/)
  return parts[0] || 'Unknown'
}
