package image

import "tsukiyo/master/internal/models"

// GetContainerTemplates 返回预制容器镜像列表 (Incus 源)
// Source 格式: images:distro/release/cloud  (从 images.linuxcontainers.org 拉取)
func GetContainerTemplates() []models.ImageTemplate {
	return []models.ImageTemplate{
		{
			ID: "ubuntu-noble", Name: "Ubuntu 24.04",
			Type: models.ImageTypeContainer, Distro: "ubuntu", Release: "24.04", Arch: "amd64",
			Description: "Ubuntu 24.04 LTS",
			URL:         "images:ubuntu/24.04/cloud",
			Enabled:     true,
		},
		{
			ID: "ubuntu-jammy", Name: "Ubuntu 22.04",
			Type: models.ImageTypeContainer, Distro: "ubuntu", Release: "22.04", Arch: "amd64",
			Description: "Ubuntu 22.04 LTS",
			URL:         "images:ubuntu/22.04/cloud",
			Enabled:     true,
		},
		{
			ID: "ubuntu-focal", Name: "Ubuntu 20.04",
			Type: models.ImageTypeContainer, Distro: "ubuntu", Release: "20.04", Arch: "amd64",
			Description: "Ubuntu 20.04 LTS",
			URL:         "images:ubuntu/20.04/cloud",
			Enabled:     true,
		},
		{
			ID: "debian-trixie", Name: "Debian 13",
			Type: models.ImageTypeContainer, Distro: "debian", Release: "trixie", Arch: "amd64",
			Description: "Debian 13 (Trixie)",
			URL:         "images:debian/trixie/cloud",
			Enabled:     true,
		},
		{
			ID: "debian-bookworm", Name: "Debian 12",
			Type: models.ImageTypeContainer, Distro: "debian", Release: "bookworm", Arch: "amd64",
			Description: "Debian 12 (Bookworm)",
			URL:         "images:debian/bookworm/cloud",
			Enabled:     true,
		},
		{
			ID: "debian-bullseye", Name: "Debian 11",
			Type: models.ImageTypeContainer, Distro: "debian", Release: "bullseye", Arch: "amd64",
			Description: "Debian 11 (Bullseye)",
			URL:         "images:debian/bullseye/cloud",
			Enabled:     true,
		},
		{
			ID: "alpine-3.21", Name: "Alpine 3.21",
			Type: models.ImageTypeContainer, Distro: "alpine", Release: "3.21", Arch: "amd64",
			Description: "Alpine Linux 3.21",
			URL:         "images:alpine/3.21/cloud",
			Enabled:     true,
		},
		{
			ID: "centos-9-stream", Name: "CentOS 9 Stream",
			Type: models.ImageTypeContainer, Distro: "centos", Release: "9-Stream", Arch: "amd64",
			Description: "CentOS 9 Stream",
			URL:         "images:centos/9-Stream/cloud",
			Enabled:     true,
		},
		{
			ID: "archlinux-current", Name: "Arch Linux",
			Type: models.ImageTypeContainer, Distro: "archlinux", Release: "current", Arch: "amd64",
			Description: "Arch Linux (Rolling)",
			URL:         "images:archlinux/current/cloud",
			Enabled:     true,
		},
		{
			ID: "fedora-42", Name: "Fedora 42",
			Type: models.ImageTypeContainer, Distro: "fedora", Release: "42", Arch: "amd64",
			Description: "Fedora 42",
			URL:         "images:fedora/42/cloud",
			Enabled:     true,
		},
		{
			ID: "rockylinux-9", Name: "Rocky Linux 9",
			Type: models.ImageTypeContainer, Distro: "rockylinux", Release: "9", Arch: "amd64",
			Description: "Rocky Linux 9",
			URL:         "images:rockylinux/9/cloud",
			Enabled:     true,
		},
		{
			ID: "almalinux-9", Name: "AlmaLinux 9",
			Type: models.ImageTypeContainer, Distro: "almalinux", Release: "9", Arch: "amd64",
			Description: "AlmaLinux 9",
			URL:         "images:almalinux/9/cloud",
			Enabled:     true,
		},
		{
			ID: "opensuse-15.6", Name: "openSUSE Leap 15.6",
			Type: models.ImageTypeContainer, Distro: "opensuse", Release: "15.6", Arch: "amd64",
			Description: "openSUSE Leap 15.6",
			URL:         "images:opensuse/15.6/cloud",
			Enabled:     true,
		},
	}
}

// GetVMTemplates 返回预制虚拟机镜像列表 (各发行版 cloud image 下载)
func GetVMTemplates() []models.ImageTemplate {
	return []models.ImageTemplate{
		{
			ID: "kvm-ubuntu-noble", Name: "Ubuntu 24.04 KVM",
			Type: models.ImageTypeVM, Distro: "ubuntu", Release: "24.04", Arch: "amd64",
			Description: "Ubuntu 24.04 LTS cloud image",
			URL:         "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			Enabled:     true,
		},
		{
			ID: "kvm-ubuntu-noble-xfce", Name: "Ubuntu 24.04 XFCE KVM",
			Type: models.ImageTypeVM, Distro: "ubuntu", Release: "24.04", Arch: "amd64",
			Description: "Ubuntu 24.04 LTS + XFCE 桌面 (cloud-init)",
			URL:         "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			Desktop:     "xfce",
			Enabled:     true,
		},
		{
			ID: "kvm-ubuntu-jammy", Name: "Ubuntu 22.04 KVM",
			Type: models.ImageTypeVM, Distro: "ubuntu", Release: "22.04", Arch: "amd64",
			Description: "Ubuntu 22.04 LTS cloud image",
			URL:         "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img",
			Enabled:     true,
		},
		{
			ID: "kvm-debian-bookworm", Name: "Debian 12 KVM",
			Type: models.ImageTypeVM, Distro: "debian", Release: "bookworm", Arch: "amd64",
			Description: "Debian 12 generic cloud image",
			URL:         "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2",
			Enabled:     true,
		},
		{
			ID: "kvm-debian-bookworm-xfce", Name: "Debian 12 XFCE KVM",
			Type: models.ImageTypeVM, Distro: "debian", Release: "bookworm", Arch: "amd64",
			Description: "Debian 12 + XFCE 桌面 (cloud-init)",
			URL:         "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2",
			Desktop:     "xfce",
			Enabled:     true,
		},
		{
			ID: "kvm-debian-bullseye", Name: "Debian 11 KVM",
			Type: models.ImageTypeVM, Distro: "debian", Release: "bullseye", Arch: "amd64",
			Description: "Debian 11 generic cloud image",
			URL:         "https://cloud.debian.org/images/cloud/bullseye/latest/debian-11-genericcloud-amd64.qcow2",
			Enabled:     true,
		},
		{
			ID: "kvm-alpine-3.21", Name: "Alpine 3.21 KVM",
			Type: models.ImageTypeVM, Distro: "alpine", Release: "3.21", Arch: "amd64",
			Description: "Alpine Linux 3.21 NoCloud cloud-init image",
			URL:         "https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/cloud/nocloud_alpine-3.21.3-x86_64-bios-cloudinit-r0.qcow2",
			Enabled:     true,
		},
		{
			ID: "kvm-centos-9-stream", Name: "CentOS Stream 9 KVM",
			Type: models.ImageTypeVM, Distro: "centos", Release: "9-stream", Arch: "amd64",
			Description: "CentOS Stream 9 GenericCloud image",
			URL:         "https://cloud.centos.org/centos/9-stream/x86_64/images/CentOS-Stream-GenericCloud-9-latest.x86_64.qcow2",
			Enabled:     true,
		},
		{
			ID: "kvm-archlinux-current", Name: "Arch Linux KVM",
			Type: models.ImageTypeVM, Distro: "archlinux", Release: "current", Arch: "amd64",
			Description: "Arch Linux (Rolling) cloud image",
			URL:         "https://geo.mirror.pkgbuild.com/images/latest/Arch-Linux-x86_64-cloudimg.qcow2",
			Enabled:     true,
		},
		{
			ID: "kvm-fedora-42", Name: "Fedora 42 KVM",
			Type: models.ImageTypeVM, Distro: "fedora", Release: "42", Arch: "amd64",
			Description: "Fedora 42 GenericCloud image",
			URL:         "https://download.fedoraproject.org/pub/fedora/linux/releases/42/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-42-1.1.x86_64.qcow2",
			Enabled:     true,
		},
		{
			ID: "kvm-rockylinux-9", Name: "Rocky Linux 9 KVM",
			Type: models.ImageTypeVM, Distro: "rockylinux", Release: "9", Arch: "amd64",
			Description: "Rocky Linux 9 GenericCloud image",
			URL:         "https://dl.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud-Base.latest.x86_64.qcow2",
			Enabled:     true,
		},
		{
			ID: "kvm-windows-10", Name: "Windows 10 KVM",
			Type: models.ImageTypeVM, Distro: "windows", Release: "10", Arch: "amd64",
			Description: "Windows 10 Enterprise LTSC (需手动上传 ISO)",
			URL:         "",
			Enabled:     true,
		},
	}
}

// GetAllTemplates 返回全部预制模板
func GetAllTemplates() []models.ImageTemplate {
	all := make([]models.ImageTemplate, 0, 30)
	all = append(all, GetContainerTemplates()...)
	all = append(all, GetVMTemplates()...)
	return all
}

// FindTemplate 根据 ID 查找模板
func FindTemplate(id string) *models.ImageTemplate {
	for _, t := range GetAllTemplates() {
		if t.ID == id {
			return &t
		}
	}
	return nil
}
