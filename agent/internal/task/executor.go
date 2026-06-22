package task

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"tsukiyo/agent/internal/config"
	"tsukiyo/agent/internal/image"
	"tsukiyo/agent/internal/incus"
	"tsukiyo/agent/internal/network"
	"tsukiyo/agent/internal/ws"
)

// Executor 任务执行器
type Executor struct {
	cfg             *config.Config
	incusClient     *incus.Client
	netManager      *network.Manager
	wsClient        *ws.Client
	downloadManager *image.DownloadManager
}

// NewExecutor 创建任务执行器
func NewExecutor(cfg *config.Config, incusClient *incus.Client, netManager *network.Manager, wsClient *ws.Client) *Executor {
	return &Executor{
		cfg:             cfg,
		incusClient:     incusClient,
		netManager:      netManager,
		wsClient:        wsClient,
		downloadManager: image.NewDownloadManager("/var/cache/tsukiyo/images"),
	}
}

// Execute 执行任务
func (e *Executor) Execute(taskType string, payload json.RawMessage) (json.RawMessage, error) {
	zap.L().Info("开始执行任务", zap.String("type", taskType))

	switch taskType {
	case "create_instance":
		return e.handleCreateInstance(payload)
	case "delete_instance":
		return e.handleDeleteInstance(payload)
	case "start_instance":
		return e.handleStartInstance(payload)
	case "stop_instance":
		return e.handleStopInstance(payload)
	case "restart_instance":
		return e.handleRestartInstance(payload)
	case "reinstall_instance":
		return e.handleReinstallInstance(payload)
	case "resize_instance":
		return e.handleResizeInstance(payload)
	case "reset_password":
		return e.handleResetPassword(payload)
	case "create_snapshot":
		return e.handleCreateSnapshot(payload)
	case "restore_snapshot":
		return e.handleRestoreSnapshot(payload)
	case "delete_snapshot":
		return e.handleDeleteSnapshot(payload)
	case "download_image":
		return e.handleDownloadImage(payload)
	case "cancel_image_download":
		return e.handleCancelImageDownload(payload)
	case "check_image":
		return e.handleCheckImage(payload)
	case "delete_image":
		return e.handleDeleteImage(payload)
	case "list_remote_images":
		return e.handleListRemoteImages(payload)
	case "sync_images":
		return e.handleSyncImages(payload)
	case "apply_network":
		return e.handleApplyNetwork(payload)
	case "apply_firewall":
		return e.handleApplyFirewall(payload)
	case "format_disk":
		return e.handleFormatDisk(payload)
	case "init_storage":
		return e.handleInitStorage(payload)
	case "create_partition":
		return e.handleCreatePartition(payload)
	case "delete_partition":
		return e.handleDeletePartition(payload)
	case "delete_storage":
		return e.handleDeleteStorage(payload)
	case "limit_network":
		return e.handleLimitNetwork(payload)
	case "limit_iops":
		return e.handleLimitIOPS(payload)
	case "migrate_instance":
		return e.handleMigrateInstance(payload)
	case "bridge_network":
		zap.L().Info("执行 Bridge 网络任务", zap.String("type", taskType))
		return e.handleBridgeNetwork(payload)
	case "bind_bridge_egress":
		zap.L().Info("执行 Bridge 出口 EIP 绑定任务")
		return e.handleBindBridgeEgress(payload)
	case "unbind_bridge_egress":
		zap.L().Info("执行 Bridge 出口 EIP 解绑任务")
		return e.handleUnbindBridgeEgress(payload)
	case "assign_eip":
		zap.L().Info("执行实例 EIP 分配任务")
		return e.handleAssignEIP(payload)
	case "release_eip":
		zap.L().Info("执行实例 EIP 释放任务")
		return e.handleReleaseEIP(payload)
	case "add_disk":
		return e.handleAddDisk(payload)
	case "delete_disk":
		return e.handleDeleteDisk(payload)
	case "resize_disk":
		return e.handleResizeDisk(payload)
	default:
		return nil, fmt.Errorf("未知任务类型: %s", taskType)
	}
}
