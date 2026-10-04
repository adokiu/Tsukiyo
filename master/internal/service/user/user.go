package user

import (
	"strconv"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"tsukiyo/master/internal/auth"
	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

// UserService 用户服务
type UserService struct{}

// NewUserService 创建用户服务
func NewUserService() *UserService {
	return &UserService{}
}

// ListUsers 获取用户列表
func (s *UserService) ListUsers(page, pageSize int, search, statusFilter string) ([]models.User, int64, error) {
	query := db.DB.Model(&models.User{})
	if search != "" {
		query = query.Where("username ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	var total int64
	query.Count(&total)

	var users []models.User
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		zap.L().Error("查询用户列表失败", zap.Error(err))
		return nil, 0, err
	}

	return users, total, nil
}

// GetUserGroups 批量获取用户组
func (s *UserService) GetUserGroups(userIDs []uint) (map[uint][]string, error) {
	if len(userIDs) == 0 {
		return make(map[uint][]string), nil
	}
	type userGroupRow struct {
		UserID    uint   `json:"user_id"`
		GroupName string `json:"group_name"`
	}
	var rows []userGroupRow
	if err := db.DB.Table("user_group_members as m").
		Select("m.user_id, g.name as group_name").
		Joins("INNER JOIN user_groups g ON g.id = m.group_id").
		Where("m.user_id IN ?", userIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint][]string)
	for _, r := range rows {
		result[r.UserID] = append(result[r.UserID], r.GroupName)
	}
	return result, nil
}

// GetUser 获取用户详情
func (s *UserService) GetUser(userID uint) (*models.User, []models.UserGroup, error) {
	var user models.User
	if err := db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, ErrUserNotFound
		}
		return nil, nil, err
	}

	// 加载用户组
	var groups []models.UserGroup
	db.DB.Raw(`
		SELECT g.* FROM user_groups g
		INNER JOIN user_group_members m ON m.group_id = g.id
		WHERE m.user_id = ?
	`, user.ID).Scan(&groups)

	return &user, groups, nil
}

// UpdateUser 更新用户
func (s *UserService) UpdateUser(userID uint, email, status string, balanceCents *int64, password, phone, qq, realNameStatus, realName, idCard string) error {
	updates := make(map[string]interface{})
	if email != "" {
		updates["email"] = email
	}
	if status != "" {
		updates["status"] = status
	}
	if balanceCents != nil {
		updates["balance_cents"] = *balanceCents
	}
	if password != "" {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		updates["password_hash"] = hash
	}
	if phone != "" {
		updates["phone"] = phone
	}
	if qq != "" {
		updates["qq"] = qq
	}
	if realNameStatus != "" {
		updates["real_name_status"] = realNameStatus
	}
	if realName != "" {
		updates["real_name"] = realName
	}
	if idCard != "" {
		updates["id_card"] = idCard
	}

	if len(updates) == 0 {
		return service.ErrNoValidUpdateFields
	}

	if err := db.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return err
	}

	return nil
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(userID uint) error {
	if err := db.DB.Where("id = ?", userID).Delete(&models.User{}).Error; err != nil {
		return err
	}
	return nil
}

// ParseUserID 解析用户 ID
func ParseUserID(idStr string) (uint, error) {
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, service.ErrInvalidUserID
	}
	return uint(userID), nil
}
