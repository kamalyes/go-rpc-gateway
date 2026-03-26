package middleware

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goswagger "github.com/kamalyes/go-config/pkg/swagger"
	"github.com/kamalyes/go-rpc-gateway/constants"
	"github.com/kamalyes/go-rpc-gateway/global"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDocumentSpecs_UsesPathsIncludeExcludeAndCollectsDefinitions(t *testing.T) {
	require.NoError(t, global.EnsureLoggerInitialized())

	middleware := newTestSwaggerMiddleware()
	middleware.serviceSpecs["MessageService"] = map[string]interface{}{
		constants.SwaggerFieldConsumes: []interface{}{"application/json"},
		constants.SwaggerFieldProduces: []interface{}{"application/json"},
		constants.SwaggerFieldPaths: map[string]interface{}{
			"/v1/messages/send": map[string]interface{}{
				constants.SwaggerFieldParameters: []interface{}{
					map[string]interface{}{"name": "x-trace-id", "in": "header", "type": "string"},
				},
				"post": map[string]interface{}{
					"operationId": "MessageService_SendMessage",
					constants.SwaggerFieldParameters: []interface{}{
						map[string]interface{}{
							"name":     "body",
							"in":       "body",
							"required": true,
							"schema": map[string]interface{}{
								constants.SwaggerFieldRef: "#/definitions/MessageSendRequest",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "ok",
							"schema": map[string]interface{}{
								constants.SwaggerFieldRef: "#/definitions/MessageSendResponse",
							},
						},
					},
					constants.SwaggerFieldTags: []interface{}{"MessageTag"},
				},
			},
		},
		constants.SwaggerFieldDefs: map[string]interface{}{
			"MessageSendRequest": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"payload": map[string]interface{}{
						constants.SwaggerFieldRef: "#/definitions/MessagePayload",
					},
				},
			},
			"MessagePayload": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{"type": "string"},
				},
			},
			"MessageSendResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string"},
				},
			},
			"UnusedMessageDefinition": map[string]interface{}{
				"type": "object",
			},
		},
		constants.SwaggerFieldTags: []interface{}{
			map[string]interface{}{"name": "MessageTag", "description": "Message APIs"},
		},
		"securityDefinitions": map[string]interface{}{
			"bearerAuth": map[string]interface{}{
				"type": "apiKey",
				"name": "Authorization",
				"in":   "header",
			},
		},
	}

	middleware.serviceSpecs["TicketService"] = map[string]interface{}{
		constants.SwaggerFieldPaths: map[string]interface{}{
			"/v1/tickets": map[string]interface{}{
				constants.SwaggerFieldParameters: []interface{}{
					map[string]interface{}{"name": "tenant", "in": "header", "type": "string"},
				},
				"get": map[string]interface{}{
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "ok",
							"schema": map[string]interface{}{
								constants.SwaggerFieldRef: "#/definitions/TicketListResponse",
							},
						},
					},
					constants.SwaggerFieldTags: []interface{}{"TicketTag"},
				},
				"post": map[string]interface{}{
					constants.SwaggerFieldParameters: []interface{}{
						map[string]interface{}{
							"name": "body",
							"in":   "body",
							"schema": map[string]interface{}{
								constants.SwaggerFieldRef: "#/definitions/TicketCreateRequest",
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "ok",
							"schema": map[string]interface{}{
								constants.SwaggerFieldRef: "#/definitions/TicketResponse",
							},
						},
					},
					constants.SwaggerFieldTags: []interface{}{"TicketTag"},
				},
				"delete": map[string]interface{}{
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "ok",
							"schema": map[string]interface{}{
								constants.SwaggerFieldRef: "#/definitions/TicketDeleteResponse",
							},
						},
					},
					constants.SwaggerFieldTags: []interface{}{"TicketTag"},
				},
			},
		},
		constants.SwaggerFieldDefs: map[string]interface{}{
			"TicketListResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"items": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							constants.SwaggerFieldRef: "#/definitions/TicketResponse",
						},
					},
				},
			},
			"TicketCreateRequest": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{"type": "string"},
				},
			},
			"TicketResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string"},
				},
			},
			"TicketDeleteResponse": map[string]interface{}{
				"type": "object",
			},
			"UnusedTicketDefinition": map[string]interface{}{
				"type": "object",
			},
		},
		constants.SwaggerFieldTags: []interface{}{
			map[string]interface{}{"name": "TicketTag", "description": "Ticket APIs"},
		},
	}

	middleware.config.Aggregate.Documents = []*goswagger.DocumentSpec{
		{
			Name:        "open-service-demo",
			Title:       "Open Service Demo",
			Description: "Subset of message and ticket APIs",
			Version:     "1.2.3",
			Enabled:     true,
			Sources: []*goswagger.DocumentSource{
				{
					Service: "MessageService",
					Paths: []*goswagger.DocumentPathSelector{
						{Path: "/v1/messages/send"},
					},
				},
				{
					Service: "TicketService",
					Include: []*goswagger.DocumentPathSelector{
						{Path: "/v1/tickets", Methods: []string{"GET", "POST", "DELETE"}},
					},
					Exclude: []*goswagger.DocumentPathSelector{
						{Path: "/v1/tickets", Methods: []string{"delete"}},
					},
				},
			},
		},
	}

	require.NoError(t, middleware.buildDocumentSpecs())

	spec, exists := middleware.documentSpecs["open-service-demo"]
	require.True(t, exists)

	info := spec[constants.SwaggerFieldInfo].(map[string]interface{})
	assert.Equal(t, "Open Service Demo", info[constants.SwaggerFieldTitle])
	assert.Equal(t, "1.2.3", info[constants.SwaggerFieldVersion])

	paths := spec[constants.SwaggerFieldPaths].(map[string]interface{})
	require.Contains(t, paths, "/v1/messages/send")
	require.Contains(t, paths, "/v1/tickets")

	messagePath := paths["/v1/messages/send"].(map[string]interface{})
	assert.Contains(t, messagePath, "post")
	assert.Contains(t, messagePath, constants.SwaggerFieldParameters)

	ticketPath := paths["/v1/tickets"].(map[string]interface{})
	assert.Contains(t, ticketPath, "get")
	assert.Contains(t, ticketPath, "post")
	assert.NotContains(t, ticketPath, "delete")
	assert.Contains(t, ticketPath, constants.SwaggerFieldParameters)

	definitions := spec[constants.SwaggerFieldDefs].(map[string]interface{})
	assert.Contains(t, definitions, "MessageSendRequest")
	assert.Contains(t, definitions, "MessagePayload")
	assert.Contains(t, definitions, "MessageSendResponse")
	assert.Contains(t, definitions, "TicketListResponse")
	assert.Contains(t, definitions, "TicketCreateRequest")
	assert.Contains(t, definitions, "TicketResponse")
	assert.NotContains(t, definitions, "TicketDeleteResponse")
	assert.NotContains(t, definitions, "UnusedMessageDefinition")
	assert.NotContains(t, definitions, "UnusedTicketDefinition")

	securityDefinitions := spec["securityDefinitions"].(map[string]interface{})
	assert.Contains(t, securityDefinitions, "bearerAuth")

	tags := spec[constants.SwaggerFieldTags].([]interface{})
	assert.Len(t, tags, 2)

	documentInfo := spec[constants.SwaggerFieldXDocumentInfo].(map[string]interface{})
	assert.Equal(t, "open-service-demo", documentInfo[constants.SwaggerFieldName])
	assert.Equal(t, []string{"MessageService", "TicketService"}, documentInfo[constants.SwaggerFieldServices])
}

func TestBuildDocumentSpecs_DefaultsToWholeServiceWhenIncludeEmpty(t *testing.T) {
	require.NoError(t, global.EnsureLoggerInitialized())

	middleware := newTestSwaggerMiddleware()
	middleware.serviceSpecs["OpenPlatformService"] = map[string]interface{}{
		constants.SwaggerFieldPaths: map[string]interface{}{
			"/v1/open-platform/apps": map[string]interface{}{
				"get": map[string]interface{}{
					constants.SwaggerFieldTags: []interface{}{"OpenPlatformTag"},
				},
				"post": map[string]interface{}{
					constants.SwaggerFieldTags: []interface{}{"OpenPlatformTag"},
				},
			},
			"/v1/open-platform/apps/{id}": map[string]interface{}{
				"get": map[string]interface{}{
					constants.SwaggerFieldTags: []interface{}{"OpenPlatformTag"},
				},
				"delete": map[string]interface{}{
					constants.SwaggerFieldTags: []interface{}{"OpenPlatformTag"},
				},
			},
		},
		constants.SwaggerFieldTags: []interface{}{
			map[string]interface{}{"name": "OpenPlatformTag"},
		},
	}

	middleware.config.Aggregate.Documents = []*goswagger.DocumentSpec{
		{
			Name:    "open-platform-public",
			Enabled: true,
			Sources: []*goswagger.DocumentSource{
				{
					Service: "OpenPlatformService",
					Exclude: []*goswagger.DocumentPathSelector{
						{Path: "/v1/open-platform/apps/{id}", Methods: []string{"delete"}},
					},
				},
			},
		},
	}

	require.NoError(t, middleware.buildDocumentSpecs())

	spec, exists := middleware.documentSpecs["open-platform-public"]
	require.True(t, exists)

	paths := spec[constants.SwaggerFieldPaths].(map[string]interface{})
	require.Contains(t, paths, "/v1/open-platform/apps")
	require.Contains(t, paths, "/v1/open-platform/apps/{id}")

	appsPath := paths["/v1/open-platform/apps"].(map[string]interface{})
	assert.Contains(t, appsPath, "get")
	assert.Contains(t, appsPath, "post")

	appDetailPath := paths["/v1/open-platform/apps/{id}"].(map[string]interface{})
	assert.Contains(t, appDetailPath, "get")
	assert.NotContains(t, appDetailPath, "delete")
}

func TestSwaggerUIIncludesCommonNavigationLinks(t *testing.T) {
	middleware := newTestSwaggerMiddleware()

	rootHTML := middleware.generateRootSwaggerUI()
	serviceHTML := middleware.generateServiceSwaggerUI("MessageService")
	documentHTML := middleware.generateDocumentSwaggerUI("open-service-demo", "Open Service Demo")

	for _, html := range []string{rootHTML, serviceHTML, documentHTML} {
		assert.True(t, strings.Contains(html, "返回文档列表"))
		assert.True(t, strings.Contains(html, "查看服务列表"))
		assert.True(t, strings.Contains(html, "查看聚合文档"))
		assert.True(t, strings.Contains(html, `/swagger/documents`))
		assert.True(t, strings.Contains(html, `/swagger/services`))
		assert.True(t, strings.Contains(html, `href="/swagger"`))
	}
}

func TestUpdateConfig_RebuildsDocumentsFromLatestAggregateConfig(t *testing.T) {
	require.NoError(t, global.EnsureLoggerInitialized())

	tempDir := t.TempDir()
	serviceSpecPath := filepath.Join(tempDir, "message.swagger.yaml")
	serviceSpec := `swagger: "2.0"
info:
  title: "Message Service"
  version: "1.0.0"
paths:
  /v1/messages/history:
    get:
      tags: ["MessageTag"]
      responses:
        "200":
          description: "ok"
  /v1/messages/send:
    post:
      tags: ["MessageTag"]
      responses:
        "200":
          description: "ok"
tags:
  - name: "MessageTag"
`
	require.NoError(t, os.WriteFile(serviceSpecPath, []byte(serviceSpec), 0o600))

	initialConfig := goswagger.Default()
	initialConfig.Enabled = true
	initialConfig.HotReload = false
	initialConfig.Aggregate.Enabled = true
	initialConfig.Aggregate.Services = []*goswagger.ServiceSpec{
		{
			Name:     "MessageService",
			SpecPath: serviceSpecPath,
			Enabled:  true,
		},
	}
	initialConfig.Aggregate.Documents = []*goswagger.DocumentSpec{
		{
			Name:    "open-service",
			Enabled: true,
			Sources: []*goswagger.DocumentSource{
				{
					Service: "MessageService",
					Include: []*goswagger.DocumentPathSelector{
						{Path: "/v1/messages/history"},
					},
				},
			},
		},
	}

	middleware := newTestSwaggerMiddleware()
	require.NoError(t, middleware.UpdateConfig(initialConfig))

	initialSpec, exists := middleware.documentSpecs["open-service"]
	require.True(t, exists)
	initialPaths := initialSpec[constants.SwaggerFieldPaths].(map[string]interface{})
	assert.Contains(t, initialPaths, "/v1/messages/history")
	assert.NotContains(t, initialPaths, "/v1/messages/send")

	updatedConfig := goswagger.Default()
	updatedConfig.Enabled = true
	updatedConfig.HotReload = false
	updatedConfig.Aggregate.Enabled = true
	updatedConfig.Aggregate.Services = []*goswagger.ServiceSpec{
		{
			Name:     "MessageService",
			SpecPath: serviceSpecPath,
			Enabled:  true,
		},
	}
	updatedConfig.Aggregate.Documents = []*goswagger.DocumentSpec{
		{
			Name:    "open-service",
			Enabled: true,
			Sources: []*goswagger.DocumentSource{
				{
					Service: "MessageService",
					Include: []*goswagger.DocumentPathSelector{
						{Path: "/v1/messages/send"},
					},
				},
			},
		},
	}

	require.NoError(t, middleware.UpdateConfig(updatedConfig))

	updatedSpec, exists := middleware.documentSpecs["open-service"]
	require.True(t, exists)
	updatedPaths := updatedSpec[constants.SwaggerFieldPaths].(map[string]interface{})
	assert.NotContains(t, updatedPaths, "/v1/messages/history")
	assert.Contains(t, updatedPaths, "/v1/messages/send")
}

func newTestSwaggerMiddleware() *SwaggerMiddleware {
	cfg := goswagger.Default()
	cfg.Enabled = true
	cfg.Version = "9.9.9"
	cfg.Aggregate.Enabled = true

	return &SwaggerMiddleware{
		config:        cfg,
		serviceSpecs:  make(map[string]map[string]interface{}),
		documentSpecs: make(map[string]map[string]interface{}),
		lastUpdated:   time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
	}
}
