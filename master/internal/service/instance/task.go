package instance

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
	"tsukiyo/master/internal/service"
)

// TaskService 任务服务
type TaskService struct{}

// NewTaskService 创建任务服务
func NewTaskService() *TaskService {
	return &TaskService{}
}

// ListTasksRequest 获取任务列表请求
type ListTasksRequest struct {
	Page    int
	PerPage int
	Search  string
	Status  string
	NodeID  uuid.UUID
	Filters map[string]string
}

// ListTasks 获取任务列表
func (s *TaskService) ListTasks(req ListTasksRequest) ([]models.Task, int64, error) {
	query := db.DB.Model(&models.Task{})
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.NodeID != uuid.Nil {
		query = query.Where("node_id = ?", req.NodeID)
	}

	// 搜索：匹配 type
	if req.Search != "" {
		searchPattern := "%" + req.Search + "%"
		query = query.Where("type ILIKE ? OR CAST(id AS TEXT) ILIKE ?", searchPattern, searchPattern)
	}

	// 筛选
	if v, ok := req.Filters["type"]; ok && v != "" {
		query = query.Where("type = ?", v)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tasks []models.Task
	offset := (req.Page - 1) * req.PerPage
	if err := query.Preload("Node").Preload("Instance").Order("created_at DESC").Limit(req.PerPage).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// GetTask 获取任务详情
func (s *TaskService) GetTask(taskID uuid.UUID) (*models.Task, error) {
	var task models.Task
	if err := db.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, &service.ServiceError{Message: "任务不存在"}
		}
		return nil, err
	}
	return &task, nil
}

// GetTaskLogs 获取任务日志
func (s *TaskService) GetTaskLogs(taskID uuid.UUID) ([]models.TaskLog, error) {
	var logs []models.TaskLog
	if err := db.DB.Where("task_id = ?", taskID).Order("created_at ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
