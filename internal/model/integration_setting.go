package model

import "time"

// IntegrationSetting 外部服务配置。敏感值在写入存储前由服务层加密。
type IntegrationSetting struct {
	Section   string
	Key       string
	Value     string
	Secret    bool
	UpdatedAt time.Time
}
