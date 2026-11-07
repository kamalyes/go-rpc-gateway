/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\aggregate\load.go
 * @Description: 聚合服务加载 - 单服务规范加载、文件/URL 双源加载
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package aggregate

import (
	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	loader "github.com/kamalyes/go-rpc-gateway/middleware/swagger/loader"
)

// loadSingleService 加载单个服务规范
func (a *Aggregator) loadSingleService(index int, service *goswagger.ServiceSpec, loadedServices map[string]bool) {
	a.logger.Info("正在处理第 %d 个服务: %s (enabled: %t, spec_path: %s)",
		index+1, service.Name, service.Enabled, service.SpecPath)

	if !service.Enabled {
		a.logger.Info("跳过已禁用的服务: %s", service.Name)
		return
	}

	if loadedServices[service.Name] {
		a.logger.Warn("服务 %s 已存在，跳过重复加载", service.Name)
		return
	}

	spec := a.loadServiceSpec(service)
	if spec == nil {
		return
	}

	if err := a.processAndStoreSpec(service, spec); err != nil {
		a.logger.Error("处理服务 %s 的规范失败: %v", service.Name, err)
		return
	}

	loadedServices[service.Name] = true
	a.logger.Info("成功加载服务 %s 的规范", service.Name)
}

// loadServiceSpec 加载服务规范（依次尝试文件和 URL）
func (a *Aggregator) loadServiceSpec(service *goswagger.ServiceSpec) map[string]interface{} {
	if spec := a.tryLoadFromFile(service); spec != nil {
		return spec
	}
	if spec := a.tryLoadFromURL(service); spec != nil {
		return spec
	}
	a.logger.Error("无法加载服务 %s 的规范：文件和URL都失败", service.Name)
	return nil
}

// tryLoadFromFile 尝试从文件加载服务规范
func (a *Aggregator) tryLoadFromFile(service *goswagger.ServiceSpec) map[string]interface{} {
	if service.SpecPath == "" {
		return nil
	}
	a.logger.Info("尝试从文件加载服务 %s 的规范: %s", service.Name, service.SpecPath)
	spec, err := loader.LoadSpecFromFile(service.SpecPath)
	if err != nil {
		a.logger.Error("从文件加载服务 %s 的规范失败: %v", service.Name, err)
		return nil
	}
	a.logger.Info("成功从文件加载服务 %s 的规范", service.Name)
	return spec
}

// tryLoadFromURL 尝试从 URL 加载服务规范
func (a *Aggregator) tryLoadFromURL(service *goswagger.ServiceSpec) map[string]interface{} {
	if service.URL == "" {
		return nil
	}
	a.logger.Info("尝试从URL加载服务 %s 的规范: %s", service.Name, service.URL)
	spec, err := loader.LoadSpecFromURL(a.httpClient, service.URL)
	if err != nil {
		a.logger.Error("从URL加载服务 %s 的规范失败: %v", service.Name, err)
		return nil
	}
	a.logger.Info("成功从URL加载服务 %s 的规范", service.Name)
	return spec
}
