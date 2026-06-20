package geoip

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var client = &http.Client{Timeout: 5 * time.Second}

type ipAPIResponse struct {
	Status      string `json:"status"`
	CountryCode string `json:"countryCode"`
}

// LookupCountryCode 通过 IP 地址查询国家码（ISO 3166-1 alpha-2）
// 使用 ip-api.com 在线 API，每分钟限 45 次请求
func LookupCountryCode(ip string) string {
	if ip == "" {
		return ""
	}

	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode", ip))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var apiResp ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return ""
	}

	if apiResp.Status != "success" {
		return ""
	}

	return apiResp.CountryCode
}
