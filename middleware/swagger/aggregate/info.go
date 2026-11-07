/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\info.go
 * @Description: 聚合信息构建 - 聚合文档 info、contact、license、services 摘要
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import (
	"time"

	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	safe "github.com/kamalyes/go-toolbox/pkg/safe"
)

// buildAggregateInfo 构建聚合文档 info 字段
func (a *Aggregator) buildAggregateInfo() map[string]interface{} {
	info := map[string]interface{}{
		constants.FieldTitle:       a.config.Title,
		constants.FieldDescription: a.config.Description,
		constants.FieldVersion:     a.config.Version,
	}

	if contact := a.buildContactInfo(); contact != nil {
		info[constants.FieldContact] = contact
	}
	if license := a.buildLicenseInfo(); license != nil {
		info[constants.FieldLicense] = license
	}

	return info
}

// buildContactInfo 构建联系信息，仅添加非空字段
func (a *Aggregator) buildContactInfo() interface{} {
	safeContact := safe.New(a.config.Contact)
	contact := make(map[string]interface{})

	if name := safeContact.Field("Name").String(""); name != "" {
		contact[constants.FieldName] = name
	}
	if email := safeContact.Field("Email").String(""); email != "" {
		contact[constants.FieldEmail] = email
	}
	if url := safeContact.Field("URL").String(""); url != "" {
		contact[constants.FieldURL] = url
	}

	if len(contact) > 0 {
		return contact
	}
	return nil
}

// buildLicenseInfo 构建许可证信息，仅添加非空字段
func (a *Aggregator) buildLicenseInfo() interface{} {
	safeLicense := safe.New(a.config.License)
	license := make(map[string]interface{})

	if name := safeLicense.Field("Name").String(""); name != "" {
		license[constants.FieldName] = name
	}
	if url := safeLicense.Field("URL").String(""); url != "" {
		license[constants.FieldURL] = url
	}

	if len(license) > 0 {
		return license
	}
	return nil
}

// buildServicesSummary 构建已启用服务的摘要列表
func (a *Aggregator) buildServicesSummary() []interface{} {
	var services []interface{}
	for _, service := range a.config.Aggregate.Services {
		if service.Enabled {
			serviceInfo := map[string]interface{}{
				constants.FieldName:        service.Name,
				constants.FieldDescription: service.Description,
				constants.FieldVersion:     service.Version,
				constants.FieldTags:        service.Tags,
				constants.FieldEnabled:     service.Enabled,
			}
			services = append(services, serviceInfo)
		}
	}
	return services
}

// buildServicesInfo 构建聚合服务元信息
func (a *Aggregator) buildServicesInfo() map[string]interface{} {
	return map[string]interface{}{
		constants.FieldMode:     a.config.Aggregate.Mode,
		constants.FieldServices: a.buildServicesSummary(),
		constants.FieldUpdated:  a.lastUpdated.Format(time.RFC3339),
		constants.FieldCount:    len(a.serviceSpecs),
	}
}
