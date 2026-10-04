package middleware

import (
	"tsukiyo/master/internal/db"
)

// UserHasAdminPanelAccess 是否可访问管理端 API / 管理端 WebSocket（admin 组或拥有 system:config）。
func UserHasAdminPanelAccess(userID uint) bool {
	if userID == 0 {
		return false
	}

	var adminCount int64
	db.DB.Raw(`
		SELECT COUNT(*) FROM user_group_members m
		INNER JOIN user_groups g ON g.id = m.group_id
		WHERE m.user_id = ? AND g.name = 'admin'
	`, userID).Scan(&adminCount)
	if adminCount > 0 {
		return true
	}

	var permCount int64
	db.DB.Raw(`
		SELECT COUNT(*) FROM group_permissions gp
		INNER JOIN user_group_members m ON m.group_id = gp.group_id
		WHERE m.user_id = ? AND gp.permission_id = 'system:config'
	`, userID).Scan(&permCount)
	return permCount > 0
}
