package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"tsukiyo/master/internal/db"
	"tsukiyo/master/internal/models"
)

// GetDashboard 获取仪表盘数据
func GetDashboard(c *gin.Context) {
	var totalUsers, totalNodes, onlineNodes, totalInstances, runningInstances int64

	db.DB.Model(&models.User{}).Count(&totalUsers)
	db.DB.Model(&models.Node{}).Count(&totalNodes)
	db.DB.Model(&models.Node{}).Where("status = ?", models.NodeStatusOnline).Count(&onlineNodes)
	db.DB.Model(&models.Instance{}).Count(&totalInstances)
	db.DB.Model(&models.Instance{}).Where("status = ?", models.InstanceStatusRunning).Count(&runningInstances)

	// 最近任务
	var recentTasks []models.Task
	db.DB.Order("created_at DESC").Limit(10).Find(&recentTasks)

	taskList := make([]gin.H, 0, len(recentTasks))
	for _, t := range recentTasks {
		taskList = append(taskList, gin.H{
			"id":     t.ID.String(),
			"type":   t.Type,
			"status": t.Status,
			"error":  t.Error,
		})
	}

	// 节点资源
	var nodes []models.Node
	db.DB.Order("created_at DESC").Find(&nodes)

	nodeResources := make([]gin.H, 0, len(nodes))
	for _, node := range nodes {
		nodeResources = append(nodeResources, gin.H{
			"id":            node.ID.String(),
			"name":          node.Name,
			"status":        node.Status,
			"cpu_percent":   node.UsedCPU,
			"mem_percent":   float64(node.UsedMemory) / float64(node.TotalMemory) * 100,
			"disk_percent":  float64(node.UsedDisk) / float64(node.TotalDisk) * 100,
			"instance_count": node.InstanceCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total_users":       totalUsers,
		"total_nodes":       totalNodes,
		"online_nodes":      onlineNodes,
		"total_instances":   totalInstances,
		"running_instances": runningInstances,
		"recent_tasks":      taskList,
		"node_resources":    nodeResources,
	})
}
