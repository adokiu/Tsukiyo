package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// ListQuery 通用列表查询参数
type ListQuery struct {
	Page    int    // 页码，从1开始
	PerPage int    // 每页数量
	Search  string // 搜索关键字
	Sort    string // 排序字段
	Order   string // 排序方向 asc/desc
	Filters map[string]string
}

// ParseListQuery 从 gin.Context 解析通用列表查询参数
func ParseListQuery(c *gin.Context) ListQuery {
	q := ListQuery{
		Page:    1,
		PerPage: 20,
		Filters: make(map[string]string),
	}

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			q.Page = parsed
		}
	}
	if pp := c.Query("per_page"); pp != "" {
		if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 200 {
			q.PerPage = parsed
		}
	}
	q.Search = c.Query("search")
	q.Sort = c.Query("sort")
	q.Order = c.Query("order")
	if q.Order == "" {
		q.Order = "desc"
	}

	// 解析所有 filter_xxx 参数
	for key, values := range c.Request.URL.Query() {
		if len(key) > 7 && key[:7] == "filter_" && len(values) > 0 && values[0] != "" {
			q.Filters[key[7:]] = values[0]
		}
	}

	return q
}

// ListResponse 通用列表响应
type ListResponse struct {
	Data    interface{} `json:"data"`
	Total   int64       `json:"total"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
}

// broadcastFn 全局广播函数，由 main.go 注入
var broadcastFn func(msgType string, payload interface{})

// SetBroadcastFn 设置全局广播函数
func SetBroadcastFn(fn func(msgType string, payload interface{})) {
	broadcastFn = fn
}

// BroadcastDataRefresh 广播数据刷新通知
// 前端收到后自动重新请求当前页数据
func BroadcastDataRefresh(resource string, nodeID string) {
	if broadcastFn == nil {
		return
	}
	broadcastFn("data_refresh", map[string]interface{}{
		"resource": resource,
		"node_id":  nodeID,
	})
}
