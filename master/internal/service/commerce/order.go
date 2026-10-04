package commerce

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service/instance"
)

// UserOrderRequest 用户下单请求
type UserOrderRequest struct {
	ProductID       string          `json:"product_id" binding:"required"`
	Name            string          `json:"name"`
	VCPU            int             `json:"vcpu"`
	MemoryMB        int             `json:"memory_mb"`
	DiskMB          int             `json:"disk_mb"`
	PaymentPeriod   string          `json:"payment_period" binding:"required,oneof=monthly quarterly half_yearly yearly"`
	TemplateID      string          `json:"template_id" binding:"required"`
	ImageKey        string          `json:"image_key"`
	NodeID          string          `json:"node_id"`
	BridgeID        string          `json:"bridge_id"`
	LoginMethod     string          `json:"login_method"`
	SSHPassword     string          `json:"ssh_password"`
	SSHPublicKey    string          `json:"ssh_public_key"`
	DataDiskMB      int             `json:"data_disk_mb"`
	NetworkDownMbps int             `json:"network_down_mbps"`
	NetworkUpMbps   int             `json:"network_up_mbps"`
	NATPortCount    int             `json:"nat_port_count"`
	IPv4EIPCount    int             `json:"ipv4_eip_count"`
	IPv6Items       []IPv6OrderItem `json:"ipv6_items"`
	Trial           bool            `json:"trial"`
}

// IPv6OrderItem IPv6下单配置项
type IPv6OrderItem struct {
	PrefixLen int `json:"prefix_len"`
	Count     int `json:"count"`
}

// UserOrderResult 用户下单结果
type UserOrderResult struct {
	InstanceID string `json:"instance_id"`
	TaskID     string `json:"task_id"`
	BillNo     string `json:"bill_no"`
}

// periodMonths 各付款周期对应月数
var periodMonths = map[string]int{
	"monthly":     1,
	"quarterly":   3,
	"half_yearly": 6,
	"yearly":      12,
}

// periodDiscount 各付款周期折扣
func periodDiscount(product *models.Product, period string) float64 {
	switch period {
	case "quarterly":
		return product.QuarterlyDiscount
	case "half_yearly":
		return product.HalfYearlyDiscount
	case "yearly":
		return product.YearlyDiscount
	default:
		return 1.0
	}
}

// CalculateOrderPrice 计算订单价格（分）
func CalculateOrderPrice(product *models.Product, req *UserOrderRequest) int64 {
	months := periodMonths[req.PaymentPeriod]
	if months == 0 {
		months = 1
	}

	// 基础价格
	base := product.BasePriceCents

	// 增配 CPU 差价
	vcpuExtra := 0
	if req.VCPU > product.VCPUMin {
		vcpuExtra = req.VCPU - product.VCPUMin
	}
	cpuExtraCents := int64(vcpuExtra) * product.VCPUUnitPriceCents

	// 增配内存差价
	memoryExtra := 0
	if req.MemoryMB > product.MemoryMinMB {
		memoryExtra = req.MemoryMB - product.MemoryMinMB
	}
	var memoryExtraCents int64
	if product.MemoryUnit == "gb" {
		memoryExtraCents = int64(memoryExtra/1024) * product.MemoryUnitPriceCents
	} else {
		memoryExtraCents = int64(memoryExtra) * product.MemoryUnitPriceCents
	}

	// 增配磁盘差价
	diskExtra := 0
	if req.DiskMB > product.DiskMinMB {
		diskExtra = req.DiskMB - product.DiskMinMB
	}
	diskExtraCents := int64(diskExtra/1024) * product.DiskUnitPriceCents

	// 数据盘差价
	dataDiskExtraCents := int64(0)
	if req.DataDiskMB > 0 && product.DataDiskAllow {
		dataDiskExtraCents = int64(req.DataDiskMB/1024) * product.DataDiskUnitPriceCents
	}

	// NAT 端口增配差价
	natPortExtraCents := int64(0)
	if product.IPv4Mode == "nat" && req.NATPortCount > product.NATPortMin {
		natPortExtra := req.NATPortCount - product.NATPortMin
		natPortExtraCents = int64(natPortExtra) * product.NATPortUnitPriceCents
	}

	// 下行带宽增配差价
	netDownExtraCents := int64(0)
	if req.NetworkDownMbps > product.NetworkDownMinMbps {
		netDownExtraCents = int64(req.NetworkDownMbps-product.NetworkDownMinMbps) * product.NetworkDownUnitPriceCents
	}

	// 上行带宽增配差价
	netUpExtraCents := int64(0)
	if req.NetworkUpMbps > product.NetworkUpMinMbps {
		netUpExtraCents = int64(req.NetworkUpMbps-product.NetworkUpMinMbps) * product.NetworkUpUnitPriceCents
	}

	// IPv4 独立IP增配差价
	ipv4EIPExtraCents := int64(0)
	if product.IPv4Mode == "eip" && req.IPv4EIPCount > product.IPv4EIPMin {
		ipv4EIPExtra := req.IPv4EIPCount - product.IPv4EIPMin
		ipv4EIPExtraCents = int64(ipv4EIPExtra) * product.IPv4EIPUnitPriceCents
	}

	// IPv6 增配差价（按前缀长度匹配配置项计算）
	ipv6ExtraCents := int64(0)
	if product.IPv6Enabled && len(req.IPv6Items) > 0 {
		var ipv6Configs []struct {
			PrefixLen      int   `json:"prefix_len"`
			Min            int   `json:"min"`
			UnitPriceCents int64 `json:"unit_price_cents"`
		}
		json.Unmarshal(product.IPv6Configs, &ipv6Configs)
		configMap := make(map[int]struct {
			min       int
			unitPrice int64
		})
		for _, c := range ipv6Configs {
			configMap[c.PrefixLen] = struct {
				min       int
				unitPrice int64
			}{c.Min, c.UnitPriceCents}
		}
		for _, item := range req.IPv6Items {
			if cfg, ok := configMap[item.PrefixLen]; ok {
				extra := item.Count - cfg.min
				if extra > 0 {
					ipv6ExtraCents += int64(extra) * cfg.unitPrice
				}
			}
		}
	}

	// 月单价 = 基础 + 增配
	monthlyCents := base + cpuExtraCents + memoryExtraCents + diskExtraCents + dataDiskExtraCents + natPortExtraCents + netDownExtraCents + netUpExtraCents + ipv4EIPExtraCents + ipv6ExtraCents

	// 周期总价 = 月单价 * 月数 * 折扣
	discount := periodDiscount(product, req.PaymentPeriod)
	totalCents := int64(float64(monthlyCents*int64(months)) * discount)

	// 试用
	if req.Trial && product.TrialEnabled {
		totalCents = product.TrialPriceCents
	}

	return totalCents
}

// CreateUserOrder 用户从商品下单创建实例
func (s *CommerceService) CreateUserOrder(userID uint, username string, req UserOrderRequest) (*UserOrderResult, error) {
	// 查询商品
	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("无效的商品 ID")
	}
	var product models.Product
	if err := db.DB.Where("id = ? AND status = ?", productID, models.ProductStatusActive).First(&product).Error; err != nil {
		return nil, fmt.Errorf("商品不存在或已下架")
	}

	// 校验增配范围
	vcpu := req.VCPU
	if vcpu < product.VCPUMin {
		vcpu = product.VCPUMin
	}
	if product.VCPUCustomMode == "unlimited" && vcpu > product.VCPUMax {
		vcpu = product.VCPUMax
	}
	memoryMB := req.MemoryMB
	if memoryMB < product.MemoryMinMB {
		memoryMB = product.MemoryMinMB
	}
	if product.MemoryCustomMode == "unlimited" && memoryMB > product.MemoryMaxMB {
		memoryMB = product.MemoryMaxMB
	}
	diskMB := req.DiskMB
	if diskMB < product.DiskMinMB {
		diskMB = product.DiskMinMB
	}
	if product.DiskCustomMode == "unlimited" && diskMB > product.DiskMaxMB {
		diskMB = product.DiskMaxMB
	}

	// 校验带宽增配范围
	netDownMbps := req.NetworkDownMbps
	if netDownMbps < product.NetworkDownMinMbps {
		netDownMbps = product.NetworkDownMinMbps
	}
	if product.NetworkDownCustomMode == "unlimited" && netDownMbps > product.NetworkDownMaxMbps {
		netDownMbps = product.NetworkDownMaxMbps
	}
	netUpMbps := req.NetworkUpMbps
	if netUpMbps < product.NetworkUpMinMbps {
		netUpMbps = product.NetworkUpMinMbps
	}
	if product.NetworkUpCustomMode == "unlimited" && netUpMbps > product.NetworkUpMaxMbps {
		netUpMbps = product.NetworkUpMaxMbps
	}

	// 校验 NAT 端口增配范围
	natPortCount := req.NATPortCount
	if product.IPv4Mode == "nat" {
		if natPortCount < product.NATPortMin {
			natPortCount = product.NATPortMin
		}
		if product.NATPortCustomMode == "unlimited" && product.NATPortMax > product.NATPortMin && natPortCount > product.NATPortMax {
			natPortCount = product.NATPortMax
		}
	}

	// 校验 IPv4 独立IP增配范围
	ipv4EIPCount := req.IPv4EIPCount
	if product.IPv4Mode == "eip" {
		if ipv4EIPCount < product.IPv4EIPMin {
			ipv4EIPCount = product.IPv4EIPMin
		}
		if product.IPv4EIPCustomMode == "unlimited" && product.IPv4EIPMax > product.IPv4EIPMin && ipv4EIPCount > product.IPv4EIPMax {
			ipv4EIPCount = product.IPv4EIPMax
		}
	}

	// 校验 IPv6 增配范围
	ipv6Items := req.IPv6Items
	if product.IPv6Enabled && len(ipv6Items) > 0 {
		var ipv6Configs []struct {
			PrefixLen  int    `json:"prefix_len"`
			Min        int    `json:"min"`
			Max        int    `json:"max"`
			CustomMode string `json:"custom_mode"`
		}
		json.Unmarshal(product.IPv6Configs, &ipv6Configs)
		configMap := make(map[int]struct {
			min, max   int
			customMode string
		})
		for _, c := range ipv6Configs {
			configMap[c.PrefixLen] = struct {
				min, max   int
				customMode string
			}{c.Min, c.Max, c.CustomMode}
		}
		for i := range ipv6Items {
			if cfg, ok := configMap[ipv6Items[i].PrefixLen]; ok {
				if ipv6Items[i].Count < cfg.min {
					ipv6Items[i].Count = cfg.min
				}
				if cfg.customMode == "unlimited" && cfg.max > cfg.min && ipv6Items[i].Count > cfg.max {
					ipv6Items[i].Count = cfg.max
				}
			}
		}
	}

	// 确定节点：优先用户指定，否则从商品 node_ids 中自动选择
	nodeID := req.NodeID
	if nodeID == "" {
		var nodeIDs []string
		json.Unmarshal(product.NodeIDs, &nodeIDs)
		if len(nodeIDs) == 0 {
			return nil, fmt.Errorf("商品未配置可用节点")
		}
		// 找一个健康节点
		for _, nid := range nodeIDs {
			var node models.Node
			if err := db.DB.Where("id = ?", nid).First(&node).Error; err == nil && node.IsHealthy() {
				nodeID = nid
				break
			}
		}
		if nodeID == "" {
			return nil, fmt.Errorf("商品配置的节点均不可用")
		}
	}

	// 确定网桥：优先用户指定，否则从商品 node_bridges 中取
	bridgeID := req.BridgeID
	if bridgeID == "" {
		var nodeBridges map[string]string
		json.Unmarshal(product.NodeBridges, &nodeBridges)
		if bid, ok := nodeBridges[nodeID]; ok && bid != "" {
			bridgeID = bid
		}
	}

	// 计算价格
	orderReq := &req
	orderReq.VCPU = vcpu
	orderReq.MemoryMB = memoryMB
	orderReq.DiskMB = diskMB
	orderReq.NetworkDownMbps = netDownMbps
	orderReq.NetworkUpMbps = netUpMbps
	orderReq.NATPortCount = natPortCount
	orderReq.IPv4EIPCount = ipv4EIPCount
	orderReq.IPv6Items = ipv6Items
	totalCents := CalculateOrderPrice(&product, orderReq)

	// 试用模式
	var expiresAt *time.Time
	if req.Trial && product.TrialEnabled {
		exp := time.Now().Add(time.Duration(product.TrialHours) * time.Hour)
		expiresAt = &exp
	} else {
		months := periodMonths[req.PaymentPeriod]
		exp := time.Now().AddDate(0, months, 0)
		expiresAt = &exp
	}

	// 事务：扣费 + 账单 + 创建实例
	var result *UserOrderResult
	txErr := db.DB.Transaction(func(tx *gorm.DB) error {
		// 查询用户余额（加行锁）
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userID).First(&user).Error; err != nil {
			return fmt.Errorf("用户不存在")
		}

		if user.BalanceCents < totalCents {
			return fmt.Errorf("余额不足，需要 ¥%.2f，当前余额 ¥%.2f", float64(totalCents)/100, float64(user.BalanceCents)/100)
		}

		balanceBefore := user.BalanceCents
		balanceAfter := balanceBefore - totalCents

		// 扣减余额
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("balance_cents", balanceAfter).Error; err != nil {
			return fmt.Errorf("扣减余额失败: %w", err)
		}

		// 创建账单
		billNo := fmt.Sprintf("P%s%06d", time.Now().Format("20060102150405"), userID)
		description := fmt.Sprintf("购买商品: %s (%s)", product.Name, req.PaymentPeriod)
		if req.Trial && product.TrialEnabled {
			description = fmt.Sprintf("试用商品: %s (%dh)", product.Name, product.TrialHours)
		}
		bill := models.Bill{
			BillNo:        billNo,
			UserID:        userID,
			Username:      username,
			Type:          models.BillTypeConsume,
			Status:        models.BillStatusPaid,
			AmountCents:   totalCents,
			BalanceBefore: balanceBefore,
			BalanceAfter:  balanceAfter,
			Description:   description,
		}
		if err := tx.Create(&bill).Error; err != nil {
			return fmt.Errorf("创建账单失败: %w", err)
		}

		// 钱包流水
		txRecord := models.WalletTransaction{
			UserID:      userID,
			AmountCents: -totalCents,
			Type:        "consume",
			BillID:      &bill.ID,
			Description: description,
		}
		if err := tx.Create(&txRecord).Error; err != nil {
			return fmt.Errorf("记录流水失败: %w", err)
		}

		// 从商品配置解析存储池
		var diskStoragePools map[string]string
		json.Unmarshal(product.DiskStoragePools, &diskStoragePools)
		storagePool := diskStoragePools[nodeID]

		var dataDiskStoragePools map[string]string
		json.Unmarshal(product.DataDiskStoragePools, &dataDiskStoragePools)
		dataDiskStoragePool := dataDiskStoragePools[nodeID]

		// 构建创建实例请求
		loginMethod := req.LoginMethod
		if loginMethod == "" {
			loginMethod = "auto"
		}
		createReq := instance.CreateInstanceRequest{
			Name:            generateInstanceName(req.Name),
			Type:            string(product.Type),
			TemplateID:      req.TemplateID,
			ImageKey:        req.ImageKey,
			NodeID:          nodeID,
			BridgeID:        bridgeID,
			AssignToUserID:  userID,
			LoginMethod:     loginMethod,
			VCPU:            float64(vcpu),
			MemoryMB:        memoryMB,
			DiskMB:          diskMB,
			StoragePool:     storagePool,
			SSHPassword:     req.SSHPassword,
			SSHPublicKey:    req.SSHPublicKey,
			NetworkDownMbps: netDownMbps,
			NetworkUpMbps:   netUpMbps,
			SnapshotLimit:   3,
			ExpiresAt:       expiresAt,
		}

		// 数据盘
		if req.DataDiskMB > 0 && product.DataDiskAllow {
			createReq.DataDisks = []instance.DataDiskRequest{
				{
					Name:        "datadisk1",
					SizeMB:      req.DataDiskMB,
					StoragePool: dataDiskStoragePool,
					MountPoint:  "/mnt/data",
				},
			}
		}

		// 流量
		if product.TrafficMinGB > 0 {
			createReq.MonthlyTrafficGB = int64(product.TrafficMinGB)
		}
		createReq.TrafficMode = string(product.TrafficCalcMode)

		// IPv4 模式
		if product.IPv4Mode == "eip" {
			createReq.AssignEIPv4 = true
			createReq.EIPv4Count = ipv4EIPCount
			if createReq.EIPv4Count < 1 {
				createReq.EIPv4Count = 1
			}
		}

		// IPv6
		if product.IPv6Enabled {
			// 解析 IPv6Configs，取第一个配置的前缀长度作为默认
			var ipv6Configs []struct {
				PrefixLen int `json:"prefix_len"`
				Min       int `json:"min"`
			}
			json.Unmarshal(product.IPv6Configs, &ipv6Configs)
			ipv6Count := 0
			ipv6PrefixLen := 128
			if len(ipv6Configs) > 0 {
				ipv6PrefixLen = ipv6Configs[0].PrefixLen
				ipv6Count = ipv6Configs[0].Min
				if ipv6Count < 1 {
					ipv6Count = 1
				}
			}
			// 用户下单传入的 IPv6 配置
			if len(req.IPv6Items) > 0 {
				totalCount := 0
				for _, item := range req.IPv6Items {
					totalCount += item.Count
				}
				if totalCount > 0 {
					ipv6Count = totalCount
				}
			}
			if ipv6Count > 0 {
				createReq.AssignEIPv6 = true
				createReq.EIPv6Count = ipv6Count
				createReq.EIPv6PrefixLen = ipv6PrefixLen
			}
		}

		// NAT 端口映射
		if product.IPv4Mode == "nat" {
			if natPortCount <= 0 {
				natPortCount = product.NATPortMin
			}
			createReq.PortMappingCount = natPortCount
		}

		inst, task, err := instanceService.CreateInstance(createReq)
		if err != nil {
			return fmt.Errorf("创建实例失败: %w", err)
		}

		// 关联账单到实例
		tx.Model(&bill).Update("instance_id", &inst.ID)

		result = &UserOrderResult{
			InstanceID: inst.ID.String(),
			TaskID:     task.ID.String(),
			BillNo:     billNo,
		}
		return nil
	})

	if txErr != nil {
		zap.L().Error("用户下单失败", zap.Uint("user_id", userID), zap.Error(txErr))
		return nil, txErr
	}
	return result, nil
}

// instanceService 全局实例服务引用（由 main.go 注入）
var instanceService *instance.InstanceService

// SetInstanceService 注入实例服务
func SetInstanceService(svc *instance.InstanceService) {
	instanceService = svc
}

// generateInstanceName 生成实例名称：ser+10位随机数字（如 ser1234567890）
// 如果传入的 name 非空则直接使用
func generateInstanceName(name string) string {
	if name != "" {
		return name
	}
	const digits = "0123456789"
	b := make([]byte, 13)
	b[0] = 's'
	b[1] = 'e'
	b[2] = 'r'
	for i := 3; i < 13; i++ {
		b[i] = digits[rand.Intn(len(digits))]
	}
	return string(b)
}
