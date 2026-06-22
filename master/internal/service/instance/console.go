package instance

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tsukiyo/master/internal/console"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

// GetInstanceConsole 获取控制台直连信息（返回 Agent 地址 + Token，前端直连 Agent）
func (s *InstanceService) GetInstanceConsole(instanceID uuid.UUID, consoleType string) (map[string]interface{}, error) {
	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	if consoleType == "" {
		consoleType = "ssh"
	}
	if consoleType != "vnc" && consoleType != "ssh" {
		return nil, fmt.Errorf("不支持的控制台类型: %s", consoleType)
	}

	var node models.Node
	if err := db.DB.Where("id = ?", instance.NodeID).First(&node).Error; err != nil {
		return nil, fmt.Errorf("查询节点信息失败: %w", err)
	}

	if !node.IsOnline() {
		return nil, service.ErrNodeNotConnected
	}

	session := console.ConsoleSession{
		InstanceID: instance.ID.String(),
		NodeID:     instance.NodeID.String(),
		Type:       consoleType,
		IncusName:  instance.IncusName,
	}
	token, err := console.GenerateConsoleToken(session)
	if err != nil {
		return nil, fmt.Errorf("生成控制台 Token 失败: %w", err)
	}

	return map[string]interface{}{
		"type":        consoleType,
		"token":       token,
		"instance_id": instance.ID.String(),
		"incus_name":  instance.IncusName,
		"node_id":     instance.NodeID.String(),
		"expires_in":  30,
	}, nil
}

// GetConsoleCredentialsByToken 通过控制台 token 换取实例登录密码（供 VNC 控制台"粘贴密码"使用）。
// token 是已签发的 5 分钟控制台凭证, 持有者本就拥有该实例的完全控制权(可连接 VNC),
// 因此可凭 token 换取密码。密码不进入 URL, 避免出现在浏览器历史与日志中。
func (s *InstanceService) GetConsoleCredentialsByToken(token string) (map[string]interface{}, error) {
	session, err := console.ValidateConsoleToken(token)
	if err != nil {
		return nil, fmt.Errorf("token 无效或已过期: %w", err)
	}

	instanceID, err := uuid.Parse(session.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("无效的实例 ID: %w", err)
	}

	var instance models.Instance
	if err := db.DB.Where("id = ?", instanceID).First(&instance).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}

	return map[string]interface{}{
		"password": instance.SSHPassword,
	}, nil
}
