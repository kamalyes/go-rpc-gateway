/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-11-22 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-22 00:00:00
 * @FilePath: \go-rpc-gateway\config\swagger\swagger_test.go
 * @Description: Swagger配置测试模块
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package swagger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwagger_Default(t *testing.T) {
	config := Default()
	assert.NotNil(t, config)
	assert.Equal(t, "swagger", config.ModuleName)
	assert.False(t, config.Enabled)
	assert.Equal(t, "/swagger/doc.json", config.JSONPath)
	assert.Equal(t, "/swagger", config.UIPath)
	assert.Equal(t, "/swagger/doc.yaml", config.YamlPath)
	assert.Equal(t, "./docs/swagger.yaml", config.SpecPath)
	assert.Equal(t, "API Documentation", config.Title)
	assert.Equal(t, "API Documentation powered by Swagger UI", config.Description)
	assert.Equal(t, "1.0.0", config.Version)
	assert.NotNil(t, config.Contact)
	assert.NotNil(t, config.License)
	assert.NotNil(t, config.Auth)
	assert.Equal(t, AuthNone, config.Auth.Type)
	assert.NotNil(t, config.Aggregate)
	assert.NotNil(t, config.Aggregate.Documents)
	assert.Empty(t, config.Aggregate.Documents)
}

func TestSwagger_WithModuleName(t *testing.T) {
	config := Default()
	result := config.WithModuleName("custom-swagger")
	assert.Equal(t, "custom-swagger", result.ModuleName)
	assert.Equal(t, config, result)
}

func TestSwagger_WithEnabled(t *testing.T) {
	config := Default()
	result := config.WithEnabled(false)
	assert.False(t, result.Enabled)
	assert.Equal(t, config, result)
}

func TestSwagger_WithJSONPath(t *testing.T) {
	config := Default()
	result := config.WithJSONPath("/api/swagger.json")
	assert.Equal(t, "/api/swagger.json", result.JSONPath)
	assert.Equal(t, config, result)
}

func TestSwagger_WithUIPath(t *testing.T) {
	config := Default()
	result := config.WithUIPath("/docs")
	assert.Equal(t, "/docs", result.UIPath)
	assert.Equal(t, config, result)
}

func TestSwagger_WithYamlPath(t *testing.T) {
	config := Default()
	result := config.WithYamlPath("/api/swagger.yaml")
	assert.Equal(t, "/api/swagger.yaml", result.YamlPath)
	assert.Equal(t, config, result)
}

func TestSwagger_WithSpecPath(t *testing.T) {
	config := Default()
	result := config.WithSpecPath("./api/openapi.yaml")
	assert.Equal(t, "./api/openapi.yaml", result.SpecPath)
	assert.Equal(t, config, result)
}

func TestSwagger_WithTitle(t *testing.T) {
	config := Default()
	result := config.WithTitle("My API")
	assert.Equal(t, "My API", result.Title)
	assert.Equal(t, config, result)
}

func TestSwagger_WithDescription(t *testing.T) {
	config := Default()
	result := config.WithDescription("Custom API description")
	assert.Equal(t, "Custom API description", result.Description)
	assert.Equal(t, config, result)
}

func TestSwagger_WithVersion(t *testing.T) {
	config := Default()
	result := config.WithVersion("2.0.0")
	assert.Equal(t, "2.0.0", result.Version)
	assert.Equal(t, config, result)
}

func TestSwagger_WithContact(t *testing.T) {
	config := Default()
	contact := &Contact{
		Name:  "API Team",
		URL:   "https://example.com",
		Email: "api@example.com",
	}
	result := config.WithContact(contact)
	assert.Equal(t, contact, result.Contact)
	assert.Equal(t, config, result)
}

func TestSwagger_WithLicense(t *testing.T) {
	config := Default()
	license := &License{
		Name: "Apache 2.0",
		URL:  "https://www.apache.org/licenses/LICENSE-2.0",
	}
	result := config.WithLicense(license)
	assert.Equal(t, license, result.License)
	assert.Equal(t, config, result)
}

func TestSwagger_WithAuth(t *testing.T) {
	config := Default()
	auth := &AuthConfig{
		Type:     AuthBasic,
		Username: "admin",
		Password: "password",
	}
	result := config.WithAuth(auth)
	assert.Equal(t, auth, result.Auth)
	assert.Equal(t, config, result)
}

func TestSwagger_WithBasicAuth(t *testing.T) {
	config := Default()
	result := config.WithBasicAuth("user", "pass")
	assert.Equal(t, AuthBasic, result.Auth.Type)
	assert.Equal(t, "user", result.Auth.Username)
	assert.Equal(t, "pass", result.Auth.Password)
	assert.Equal(t, config, result)
}

func TestSwagger_WithBearerAuth(t *testing.T) {
	config := Default()
	result := config.WithBearerAuth("token123")
	assert.Equal(t, AuthBearer, result.Auth.Type)
	assert.Equal(t, "token123", result.Auth.Token)
	assert.Equal(t, config, result)
}

func TestSwagger_WithAggregate(t *testing.T) {
	config := Default()
	aggregate := &AggregateConfig{
		Enabled:  true,
		Mode:     "selector",
		UILayout: "dropdown",
	}
	result := config.WithAggregate(aggregate)
	assert.Equal(t, aggregate, result.Aggregate)
	assert.Equal(t, config, result)
}

func TestSwagger_WithAggregateServices(t *testing.T) {
	config := Default()
	services := []*ServiceSpec{
		{Name: "service1", SpecPath: "/path1"},
		{Name: "service2", SpecPath: "/path2"},
	}
	result := config.WithAggregateServices(services)
	assert.True(t, result.Aggregate.Enabled)
	assert.Equal(t, services, result.Aggregate.Services)
	assert.Equal(t, config, result)
}

func TestSwagger_AddAggregateService(t *testing.T) {
	config := Default()
	service := &ServiceSpec{Name: "test-service", SpecPath: "/test"}
	result := config.AddAggregateService(service)
	assert.True(t, result.Aggregate.Enabled)
	assert.Contains(t, result.Aggregate.Services, service)
	assert.Equal(t, config, result)
}

func TestSwagger_WithAggregateDocuments(t *testing.T) {
	config := &Swagger{}
	documents := []*DocumentSpec{
		NewDocumentSpec("open-platform"),
		NewDocumentSpec("internal-platform"),
	}

	result := config.WithAggregateDocuments(documents)

	assert.Equal(t, config, result)
	assert.NotNil(t, result.Aggregate)
	assert.Equal(t, "merge", result.Aggregate.Mode)
	assert.Equal(t, "tabs", result.Aggregate.UILayout)
	assert.NotNil(t, result.Aggregate.Services)
	assert.Equal(t, documents, result.Aggregate.Documents)
}

func TestSwagger_AddAggregateDocument(t *testing.T) {
	config := &Swagger{}
	document := NewDocumentSpec("open-platform")

	result := config.AddAggregateDocument(document)

	assert.Equal(t, config, result)
	assert.NotNil(t, result.Aggregate)
	assert.Equal(t, "merge", result.Aggregate.Mode)
	assert.Equal(t, "tabs", result.Aggregate.UILayout)
	assert.NotNil(t, result.Aggregate.Services)
	assert.Len(t, result.Aggregate.Documents, 1)
	assert.Same(t, document, result.Aggregate.Documents[0])
}

func TestSwagger_WithAggregateMode(t *testing.T) {
	config := Default()
	result := config.WithAggregateMode("selector")
	assert.Equal(t, "selector", result.Aggregate.Mode)
	assert.Equal(t, config, result)
}

func TestSwagger_WithAggregateUILayout(t *testing.T) {
	config := Default()
	result := config.WithAggregateUILayout("list")
	assert.Equal(t, "list", result.Aggregate.UILayout)
	assert.Equal(t, config, result)
}

func TestSwagger_EnableAggregate(t *testing.T) {
	config := Default()
	config.Aggregate.Enabled = false
	result := config.EnableAggregate()
	assert.True(t, result.Aggregate.Enabled)
	assert.Equal(t, config, result)
}

func TestSwagger_DisableAggregate(t *testing.T) {
	config := Default()
	result := config.DisableAggregate()
	assert.False(t, result.Aggregate.Enabled)
	assert.Equal(t, config, result)
}

func TestSwagger_IsAggregateEnabled(t *testing.T) {
	config := Default()
	assert.False(t, config.IsAggregateEnabled())

	config.EnableAggregate()
	assert.True(t, config.IsAggregateEnabled())
}

func TestSwagger_WithDefaults(t *testing.T) {
	config := &Swagger{}
	result := config.WithDefaults()
	assert.Equal(t, "/swagger/doc.json", result.JSONPath)
	assert.Equal(t, "/swagger", result.UIPath)
	assert.Equal(t, "API Documentation", result.Title)
	assert.NotNil(t, result.Auth)
}

func TestSwagger_WithBasePath(t *testing.T) {
	config := Default()
	result := config.WithBasePath("/api")
	assert.Equal(t, "/api/doc.json", result.JSONPath)
	assert.Equal(t, "/api/*any", result.UIPath)
	assert.Equal(t, config, result)
}

func TestSwagger_WithCustomPaths(t *testing.T) {
	config := Default()
	result := config.WithCustomPaths("/custom.json", "/custom-ui")
	assert.Equal(t, "/custom.json", result.JSONPath)
	assert.Equal(t, "/custom-ui", result.UIPath)
	assert.Equal(t, config, result)
}

func TestSwagger_WithDocumentInfo(t *testing.T) {
	config := Default()
	result := config.WithDocumentInfo("My Title", "My Description")
	assert.Equal(t, "My Title", result.Title)
	assert.Equal(t, "My Description", result.Description)
	assert.Equal(t, config, result)
}

func TestSwagger_Enable(t *testing.T) {
	config := Default()
	config.Enabled = false
	result := config.Enable()
	assert.True(t, result.Enabled)
	assert.Equal(t, config, result)
}

func TestSwagger_Disable(t *testing.T) {
	config := Default()
	result := config.Disable()
	assert.False(t, result.Enabled)
	assert.Equal(t, config, result)
}

func TestSwagger_IsEnabled(t *testing.T) {
	config := Default()
	assert.False(t, config.IsEnabled())

	config.Enabled = true
	assert.True(t, config.IsEnabled())
}

func TestSwagger_IsAuthEnabled(t *testing.T) {
	config := Default()
	assert.False(t, config.IsAuthEnabled())

	config.WithBasicAuth("user", "pass")
	assert.True(t, config.IsAuthEnabled())
}

func TestSwagger_GetAuthType(t *testing.T) {
	config := Default()
	assert.Equal(t, AuthNone, config.GetAuthType())

	config.WithBearerAuth("token")
	assert.Equal(t, AuthBearer, config.GetAuthType())
}

func TestSwagger_GetFullUIPath(t *testing.T) {
	config := Default()
	assert.Equal(t, "/swagger", config.GetFullUIPath())

	config.UIPath = ""
	assert.Equal(t, "/swagger/*any", config.GetFullUIPath())
}

func TestSwagger_GetFullJSONPath(t *testing.T) {
	config := Default()
	assert.Equal(t, "/swagger/doc.json", config.GetFullJSONPath())

	config.JSONPath = ""
	assert.Equal(t, "/swagger/doc.json", config.GetFullJSONPath())
}

func TestSwagger_ValidateAuth(t *testing.T) {
	config := Default()
	err := config.ValidateAuth()
	assert.NoError(t, err)

	config.WithBasicAuth("user", "")
	err = config.ValidateAuth()
	assert.Error(t, err)

	config.WithBasicAuth("user", "pass")
	err = config.ValidateAuth()
	assert.NoError(t, err)
}

func TestSwagger_Clone(t *testing.T) {
	config := Default()
	config.WithTitle("Test API").AddAggregateService(&ServiceSpec{Name: "service1"})

	clone := config.Clone()
	assert.NotNil(t, clone)

	clonedConfig, ok := clone.(*Swagger)
	assert.True(t, ok)
	assert.Equal(t, config.Title, clonedConfig.Title)

	// 验证深拷贝 - 嵌套切片
	clonedConfig.Aggregate.Services = append(clonedConfig.Aggregate.Services, &ServiceSpec{Name: "service2"})
	assert.NotEqual(t, len(config.Aggregate.Services), len(clonedConfig.Aggregate.Services))
}

func TestSwagger_Get(t *testing.T) {
	config := Default()
	result := config.Get()
	assert.NotNil(t, result)
	assert.Equal(t, config, result)
}

func TestSwagger_Set(t *testing.T) {
	config := Default()
	newConfig := &Swagger{
		ModuleName: "new-swagger",
		Enabled:    false,
		Title:      "New Title",
	}

	config.Set(newConfig)
	assert.Equal(t, "new-swagger", config.ModuleName)
	assert.False(t, config.Enabled)
	assert.Equal(t, "New Title", config.Title)
}

func TestSwagger_Validate(t *testing.T) {
	config := Default()
	err := config.Validate()
	assert.NoError(t, err)
}

func TestSwagger_Reset(t *testing.T) {
	config := Default()
	config.WithTitle("Modified").WithEnabled(false)

	result := config.Reset()
	assert.Equal(t, "API Documentation", result.Title)
	assert.False(t, result.Enabled)
	assert.Equal(t, config, result)
}

func TestNewServiceSpec(t *testing.T) {
	spec := NewServiceSpec("test-service", "Test Description", "/path/to/spec")
	assert.NotNil(t, spec)
	assert.Equal(t, "test-service", spec.Name)
	assert.Equal(t, "Test Description", spec.Description)
	assert.Equal(t, "/path/to/spec", spec.SpecPath)
	assert.True(t, spec.Enabled)
	assert.Equal(t, "1.0.0", spec.Version)
}

func TestNewRemoteServiceSpec(t *testing.T) {
	spec := NewRemoteServiceSpec("remote-service", "Remote Description", "http://example.com/swagger.json")
	assert.NotNil(t, spec)
	assert.Equal(t, "remote-service", spec.Name)
	assert.Equal(t, "Remote Description", spec.Description)
	assert.Equal(t, "http://example.com/swagger.json", spec.URL)
	assert.True(t, spec.Enabled)
}

func TestServiceSpec_WithMethods(t *testing.T) {
	spec := NewServiceSpec("test", "desc", "/path")
	spec.WithName("new-name").
		WithDescription("new-desc").
		WithVersion("2.0.0").
		WithBasePath("/api/v2").
		WithTags([]string{"tag1", "tag2"}).
		AddTag("tag3")

	assert.Equal(t, "new-name", spec.Name)
	assert.Equal(t, "new-desc", spec.Description)
	assert.Equal(t, "2.0.0", spec.Version)
	assert.Equal(t, "/api/v2", spec.BasePath)
	assert.Contains(t, spec.Tags, "tag1")
	assert.Contains(t, spec.Tags, "tag3")
}

func TestServiceSpec_EnableDisable(t *testing.T) {
	spec := NewServiceSpec("test", "desc", "/path")
	spec.Disable()
	assert.False(t, spec.Enabled)

	spec.Enable()
	assert.True(t, spec.Enabled)
}

func TestNewDocumentSource(t *testing.T) {
	source := NewDocumentSource("MessageService")

	assert.NotNil(t, source)
	assert.Equal(t, "MessageService", source.Service)
	assert.NotNil(t, source.Paths)
	assert.NotNil(t, source.Include)
	assert.NotNil(t, source.Exclude)
}

func TestNewDocumentSpec(t *testing.T) {
	spec := NewDocumentSpec("open-platform")

	assert.NotNil(t, spec)
	assert.Equal(t, "open-platform", spec.Name)
	assert.Equal(t, "open-platform", spec.Title)
	assert.Equal(t, "1.0.0", spec.Version)
	assert.True(t, spec.Enabled)
	assert.NotNil(t, spec.Sources)
	assert.Empty(t, spec.Sources)
}

func TestDocumentSpec_WithMethods(t *testing.T) {
	source := NewDocumentSource("MessageService").AddIncludePath("/v1/messages/history", "get")
	spec := NewDocumentSpec("open-platform").
		WithTitle("Open Platform").
		WithDescription("External APIs").
		WithVersion("2.0.0").
		AddSource(source)

	assert.Equal(t, "Open Platform", spec.Title)
	assert.Equal(t, "External APIs", spec.Description)
	assert.Equal(t, "2.0.0", spec.Version)
	assert.Len(t, spec.Sources, 1)
	assert.Same(t, source, spec.Sources[0])
}

func TestDocumentSpec_AddSourceInitializesNilSlice(t *testing.T) {
	spec := &DocumentSpec{}
	source := NewDocumentSource("MessageService")

	result := spec.AddSource(source)

	assert.Equal(t, spec, result)
	assert.Len(t, spec.Sources, 1)
	assert.Same(t, source, spec.Sources[0])
}

func TestDocumentSpec_EnableDisable(t *testing.T) {
	spec := NewDocumentSpec("open-platform")

	spec.Disable()
	assert.False(t, spec.Enabled)

	spec.Enable()
	assert.True(t, spec.Enabled)
}

func TestDocumentSource_AddIncludeAndExcludePath(t *testing.T) {
	source := &DocumentSource{Service: "MessageService"}

	result := source.
		AddIncludePath("/v1/messages/history", "get", "post").
		AddExcludePath("/v1/messages/internal", "delete")

	assert.Equal(t, source, result)
	assert.Len(t, source.Include, 1)
	assert.Equal(t, "/v1/messages/history", source.Include[0].Path)
	assert.Equal(t, []string{"get", "post"}, source.Include[0].Methods)
	assert.Len(t, source.Exclude, 1)
	assert.Equal(t, "/v1/messages/internal", source.Exclude[0].Path)
	assert.Equal(t, []string{"delete"}, source.Exclude[0].Methods)
}

func TestDocumentSource_GetEffectiveInclude(t *testing.T) {
	source := NewDocumentSource("MessageService")
	source.AddIncludePath("/v1/messages/history", "get")
	effective := source.GetEffectiveInclude()
	assert.Len(t, effective, 1)
	assert.Equal(t, "/v1/messages/history", effective[0].Path)
	assert.Equal(t, []string{"get"}, effective[0].Methods)
}

func TestDocumentSource_GetEffectiveIncludeFallsBackToPaths(t *testing.T) {
	source := &DocumentSource{
		Service: "MessageService",
		Paths: []*DocumentPathSelector{
			{Path: "/v1/messages/history", Methods: []string{"get"}},
		},
	}

	effective := source.GetEffectiveInclude()

	assert.Len(t, effective, 1)
	assert.Equal(t, "/v1/messages/history", effective[0].Path)
	assert.Equal(t, []string{"get"}, effective[0].Methods)
}

func TestDocumentSource_GetEffectiveIncludeNilReceiver(t *testing.T) {
	var source *DocumentSource
	assert.Nil(t, source.GetEffectiveInclude())
}

func TestSwagger_ChainedCalls(t *testing.T) {
	config := Default().
		WithModuleName("my-swagger").
		WithTitle("My API Documentation").
		WithDescription("Comprehensive API docs").
		WithVersion("2.0.0").
		WithBasePath("/api/docs").
		WithBasicAuth("admin", "secret").
		AddAggregateService(NewServiceSpec("users", "User Service", "/users/spec")).
		AddAggregateService(NewServiceSpec("orders", "Order Service", "/orders/spec")).
		WithAggregateMode("selector").
		WithAggregateUILayout("dropdown").
		Enable()

	assert.Equal(t, "my-swagger", config.ModuleName)
	assert.Equal(t, "My API Documentation", config.Title)
	assert.Equal(t, "Comprehensive API docs", config.Description)
	assert.Equal(t, "2.0.0", config.Version)
	assert.Equal(t, AuthBasic, config.Auth.Type)
	assert.True(t, config.Aggregate.Enabled)
	assert.Equal(t, 2, len(config.Aggregate.Services))
	assert.True(t, config.Enabled)
}
