/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-03-20 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-03-20 00:00:00
 * @FilePath: \go-rpc-gateway\middleware\swagger\documents\builder.go
 * @Description: 独立子文档核心 - Builder 结构定义、构造函数、文档构建入口
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */

package documents

import (
	"fmt"
	"sort"
	"strings"
	"time"

	goswagger "github.com/kamalyes/go-rpc-gateway/config/swagger"
	constants "github.com/kamalyes/go-rpc-gateway/middleware/swagger/constants"
	serrors "github.com/kamalyes/go-rpc-gateway/middleware/swagger/errors"
	format "github.com/kamalyes/go-rpc-gateway/middleware/swagger/format"
	convert "github.com/kamalyes/go-toolbox/pkg/convert"
	errorx "github.com/kamalyes/go-toolbox/pkg/errorx"
	logger "github.com/kamalyes/go-toolbox/pkg/logger"
	mathx "github.com/kamalyes/go-toolbox/pkg/mathx"
)

// DocumentSummary 独立文档摘要，用于列表页展示
type DocumentSummary struct {
	Name        string
	Title       string
	Description string
	Version     string
	Services    []string
}

// Builder 独立子文档构建器
type Builder struct {
	config        *goswagger.Swagger
	serviceSpecs  map[string]map[string]interface{}
	documentSpecs map[string]map[string]interface{}
	lastUpdated   time.Time
	logger        logger.ILogger
}

// NewBuilder 创建独立子文档构建器
func NewBuilder(config *goswagger.Swagger, serviceSpecs map[string]map[string]interface{}, lastUpdated time.Time, logger logger.ILogger) *Builder {
	return &Builder{
		config:        config,
		serviceSpecs:  serviceSpecs,
		documentSpecs: make(map[string]map[string]interface{}),
		lastUpdated:   lastUpdated,
		logger:        logger,
	}
}

// BuildAll 构建全部独立子文档
func (b *Builder) BuildAll() error {
	b.documentSpecs = make(map[string]map[string]interface{})

	if b.config.Aggregate == nil || len(b.config.Aggregate.Documents) == 0 {
		return nil
	}

	seenDocuments := make(map[string]bool)
	for _, document := range b.config.Aggregate.Documents {
		if document == nil || !document.Enabled {
			continue
		}

		documentName := strings.TrimSpace(document.Name)
		if documentName == "" {
			return errorx.NewError(serrors.ErrTypeDocumentNameEmpty)
		}

		if seenDocuments[documentName] {
			return errorx.NewError(serrors.ErrTypeDocumentDuplicate, documentName)
		}
		seenDocuments[documentName] = true

		spec, err := b.buildSingleDocumentSpec(document)
		if err != nil {
			return errorx.NewError(serrors.ErrTypeAggregateFailed, fmt.Sprintf("构建独立文档 %s 失败: %v", documentName, err))
		}

		b.documentSpecs[documentName] = spec
		b.logger.Info("独立文档 %s 构建完成，路径数: %d", documentName, len(spec[constants.FieldPaths].(map[string]interface{})))
	}

	return nil
}

// GetDocumentSpec 获取独立文档规范 JSON
func (b *Builder) GetDocumentSpec(documentName string) ([]byte, error) {
	if !b.config.IsAggregateEnabled() {
		return nil, errorx.NewError(serrors.ErrTypeAggregateDisabled)
	}

	spec, exists := b.findNamedSpec(documentName, "文档", b.documentSpecs)
	if !exists {
		return nil, b.namedSpecNotFoundError("文档", documentName, b.documentSpecs)
	}

	return format.MarshalSpec(spec)
}

// GetDocumentSpecs 获取所有已构建的文档规范
func (b *Builder) GetDocumentSpecs() map[string]map[string]interface{} {
	return b.documentSpecs
}

// GetDocumentSummaries 收集文档列表页所需摘要
func (b *Builder) GetDocumentSummaries() []DocumentSummary {
	summaries := make([]DocumentSummary, 0)

	if b.config.Aggregate == nil {
		return summaries
	}

	for _, document := range b.config.Aggregate.Documents {
		if document == nil || !document.Enabled {
			continue
		}

		if _, exists := b.documentSpecs[document.Name]; !exists {
			continue
		}

		title := mathx.IfNotEmpty(strings.TrimSpace(document.Title), document.Name)
		description := mathx.IfNotEmpty(strings.TrimSpace(document.Description), "Subset document generated from aggregated services")
		version := mathx.IfNotEmpty(strings.TrimSpace(document.Version), strings.TrimSpace(b.config.Version))

		summaries = append(summaries, DocumentSummary{
			Name:        document.Name,
			Title:       title,
			Description: description,
			Version:     version,
			Services:    b.collectDocumentServices(document),
		})
	}

	return summaries
}

// findNamedSpec 按名称查找规范，支持忽略大小写、包含匹配
func (b *Builder) findNamedSpec(specName, specKind string, specs map[string]map[string]interface{}) (map[string]interface{}, bool) {
	if spec, exists := specs[specName]; exists {
		return spec, true
	}

	for actualName, actualSpec := range specs {
		if strings.EqualFold(specName, actualName) {
			b.logger.Info("通过忽略大小写匹配找到%s: %s -> %s", specKind, specName, actualName)
			return actualSpec, true
		}
	}

	for actualName, actualSpec := range specs {
		if strings.Contains(strings.ToLower(specName), strings.ToLower(actualName)) {
			b.logger.Info("通过包含匹配找到%s: %s -> %s", specKind, specName, actualName)
			return actualSpec, true
		}
	}

	return nil, false
}

// namedSpecNotFoundError 构造命名规范未找到的错误
func (b *Builder) namedSpecNotFoundError(specKind, specName string, specs map[string]map[string]interface{}) error {
	availableSpecs := make([]string, 0, len(specs))
	for name := range specs {
		availableSpecs = append(availableSpecs, name)
	}
	sort.Strings(availableSpecs)

	errMsg := fmt.Sprintf("%s %s 不存在。可用%s: [%s]", specKind, specName, specKind, strings.Join(availableSpecs, ", "))
	b.logger.Error(errMsg)
	return fmt.Errorf("%s", errMsg)
}

// ResolveSpecTitle 从规范中解析标题，失败时返回 fallback
func (b *Builder) ResolveSpecTitle(spec map[string]interface{}, fallback string) string {
	info, ok := spec[constants.FieldInfo].(map[string]interface{})
	if !ok {
		return fallback
	}
	return mathx.IfNotEmpty(strings.TrimSpace(convert.MustString(info[constants.FieldTitle])), fallback)
}

// collectDocumentServices 收集独立文档引用的服务名
func (b *Builder) collectDocumentServices(document *goswagger.DocumentSpec) []string {
	seen := make(map[string]bool)
	services := make([]string, 0, len(document.Sources))

	for _, source := range document.Sources {
		if source == nil {
			continue
		}

		serviceName := strings.TrimSpace(source.Service)
		if serviceName == "" || seen[serviceName] {
			continue
		}

		seen[serviceName] = true
		services = append(services, serviceName)
	}

	return services
}

// getServiceStringField 安全获取服务字段值
func getServiceStringField(service map[string]interface{}, field string) string {
	return convert.MustString(service[field])
}
