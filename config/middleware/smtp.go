/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2025-12-05 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-12-05 16:00:00
 * @FilePath: \go-rpc-gateway\config\middleware\smtp.go
 * @Description: 风控 SMTP 邮件预警配置和模板
 *
 * Copyright (c) 2025 by kamalyes, All Rights Reserved.
 */

package middleware

// EmailAlertConfig SMTP邮件预警配置
type EmailAlertConfig struct {
	// 基础配置
	Enabled     bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`             // 是否启用邮件预警
	To          []string `mapstructure:"to" yaml:"to" json:"to"`                            // 收件人列表
	CC          []string `mapstructure:"cc" yaml:"cc" json:"cc"`                            // 抄送列表
	AppName     string   `mapstructure:"app-name" yaml:"app-name" json:"appName"`           // 应用名称
	Environment string   `mapstructure:"environment" yaml:"environment" json:"environment"` // 环境标识 (dev/staging/prod)

	// 预警主题配置
	SubjectAlert string `mapstructure:"subject-alert" yaml:"subject-alert" json:"subjectAlert"` // 预警邮件主题
	SubjectBlock string `mapstructure:"subject-block" yaml:"subject-block" json:"subjectBlock"` // 封禁邮件主题

	// HTML模板配置
	TemplateAlert string `mapstructure:"template-alert" yaml:"template-alert" json:"templateAlert"` // 预警邮件HTML模板
	TemplateBlock string `mapstructure:"template-block" yaml:"template-block" json:"templateBlock"` // 封禁邮件HTML模板

	// 高级配置
	CooldownMinutes  int `mapstructure:"cooldown-minutes" yaml:"cooldown-minutes" json:"cooldownMinutes"`        // 冷却时间(分钟)，避免频繁发送相同告警
	MaxAlertsPerHour int `mapstructure:"max-alerts-per-hour" yaml:"max-alerts-per-hour" json:"maxAlertsPerHour"` // 每小时最大告警数

	// 联系人配置
	SupportEmail  string `mapstructure:"support-email" yaml:"support-email" json:"supportEmail"`    // 技术支持邮箱
	SecurityEmail string `mapstructure:"security-email" yaml:"security-email" json:"securityEmail"` // 安全团队邮箱
}

// DefaultEmailAlertConfig 返回默认邮件预警配置
func DefaultEmailAlertConfig() *EmailAlertConfig {
	return &EmailAlertConfig{
		Enabled:          false,
		To:               []string{"admin@example.com"},
		CC:               []string{},
		AppName:          "RateLimit System",
		Environment:      "production",
		SubjectAlert:     "⚠️ 风控预警 - {{.AppName}}",
		SubjectBlock:     "🚫 风控封禁 - {{.AppName}}",
		TemplateAlert:    DefaultAlertEmailTemplate,
		TemplateBlock:    DefaultBlockEmailTemplate,
		CooldownMinutes:  5,
		MaxAlertsPerHour: 20,
		SupportEmail:     "support@example.com",
		SecurityEmail:    "security@example.com",
	}
}

// DefaultAlertEmailTemplate 默认预警邮件HTML模板
const DefaultAlertEmailTemplate = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>风控预警通知</title>
    <style>
        body {
            margin: 0;
            padding: 0;
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: #f5f5f5;
        }
        .email-container {
            max-width: 600px;
            margin: 20px auto;
            background-color: #ffffff;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .email-header {
            background: linear-gradient(135deg, #ffc107 0%, #ff9800 100%);
            color: #ffffff;
            padding: 30px;
            text-align: center;
        }
        .email-header h1 {
            margin: 0;
            font-size: 28px;
            font-weight: 600;
        }
        .email-header .icon {
            font-size: 48px;
            margin-bottom: 10px;
        }
        .email-body {
            padding: 30px;
        }
        .alert-info {
            background-color: #fff3cd;
            border-left: 4px solid #ffc107;
            padding: 15px;
            margin: 20px 0;
            border-radius: 4px;
        }
        .alert-info h2 {
            margin: 0 0 10px 0;
            color: #856404;
            font-size: 18px;
        }
        .info-table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
        }
        .info-table th,
        .info-table td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #e0e0e0;
        }
        .info-table th {
            background-color: #f8f9fa;
            font-weight: 600;
            color: #333;
            width: 35%;
        }
        .info-table td {
            color: #666;
        }
        .metric-card {
            display: inline-block;
            width: 48%;
            margin: 1%;
            padding: 15px;
            background-color: #f8f9fa;
            border-radius: 6px;
            text-align: center;
            box-sizing: border-box;
        }
        .metric-card .label {
            font-size: 12px;
            color: #666;
            margin-bottom: 5px;
        }
        .metric-card .value {
            font-size: 24px;
            font-weight: bold;
            color: #ffc107;
        }
        .threshold-bar {
            width: 100%;
            height: 30px;
            background-color: #e0e0e0;
            border-radius: 15px;
            overflow: hidden;
            margin: 20px 0;
        }
        .threshold-fill {
            height: 100%;
            background: linear-gradient(90deg, #ffc107 0%, #ff9800 100%);
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-weight: bold;
            font-size: 12px;
        }
        .action-section {
            background-color: #e8f4fd;
            border-left: 4px solid #2196f3;
            padding: 15px;
            margin: 20px 0;
            border-radius: 4px;
        }
        .action-section h3 {
            margin: 0 0 10px 0;
            color: #1976d2;
            font-size: 16px;
        }
        .action-section ul {
            margin: 0;
            padding-left: 20px;
        }
        .action-section li {
            margin: 8px 0;
            color: #666;
        }
        .email-footer {
            background-color: #f8f9fa;
            padding: 20px;
            text-align: center;
            color: #999;
            font-size: 12px;
        }
        .email-footer a {
            color: #2196f3;
            text-decoration: none;
        }
        @media only screen and (max-width: 600px) {
            .metric-card {
                width: 100%;
                margin: 10px 0;
            }
        }
    </style>
</head>
<body>
    <div class="email-container">
        <!-- 邮件头部 -->
        <div class="email-header">
            <div class="icon">⚠️</div>
            <h1>风控预警通知</h1>
            <p style="margin: 10px 0 0 0; opacity: 0.9;">{{.AppName}} - {{.Environment}}</p>
        </div>

        <!-- 邮件正文 -->
        <div class="email-body">
            <!-- 预警信息 -->
            <div class="alert-info">
                <h2>⚠️ 检测到异常行为</h2>
                <p style="margin: 5px 0 0 0; color: #856404;">
                    用户 <strong>{{.UserID}}</strong> ({{.UserType}}) 的消息发送频率已达到预警阈值
                </p>
            </div>

            <!-- 详细信息表格 -->
            <table class="info-table">
                <tr>
                    <th>🆔 用户ID</th>
                    <td>{{.UserID}}</td>
                </tr>
                <tr>
                    <th>👤 用户类型</th>
                    <td>{{.UserType}}</td>
                </tr>
                <tr>
                    <th>📊 分钟计数</th>
                    <td><strong>{{.MinuteCount}}</strong> 条/分钟</td>
                </tr>
                <tr>
                    <th>📊 小时计数</th>
                    <td><strong>{{.HourCount}}</strong> 条/小时</td>
                </tr>
                <tr>
                    <th>🎯 预警阈值</th>
                    <td>{{.AlertThreshold}} 条/分钟</td>
                </tr>
                <tr>
                    <th>🚫 封禁阈值</th>
                    <td>{{.BlockThreshold}} 条/分钟</td>
                </tr>
                <tr>
                    <th>🕐 触发时间</th>
                    <td>{{.Timestamp}}</td>
                </tr>
            </table>

            <!-- 统计指标卡片 -->
            <div style="margin: 20px 0;">
                <div class="metric-card">
                    <div class="label">分钟消息数</div>
                    <div class="value">{{.MinuteCount}}</div>
                </div>
                <div class="metric-card">
                    <div class="label">小时消息数</div>
                    <div class="value">{{.HourCount}}</div>
                </div>
            </div>

            <!-- 阈值进度条 -->
            <div style="margin: 20px 0;">
                <p style="margin: 0 0 8px 0; font-size: 14px; color: #666;">预警阈值进度</p>
                <div class="threshold-bar">
                    <div class="threshold-fill" style="width: {{.AlertPercentage}}%;">
                        {{.AlertPercentage}}%
                    </div>
                </div>
            </div>

            <!-- 建议操作 -->
            <div class="action-section">
                <h3>📋 建议处理措施</h3>
                <ul>
                    <li>立即检查用户 {{.UserID}} 的行为日志</li>
                    <li>确认是否为正常业务操作还是恶意行为</li>
                    <li>如确认异常，可手动封禁该用户</li>
                    <li>监控后续行为，防止达到自动封禁阈值</li>
                    <li>如有必要，调整风控策略参数</li>
                </ul>
            </div>

            <!-- 系统信息 -->
            <div style="margin-top: 30px; padding-top: 20px; border-top: 1px solid #e0e0e0;">
                <p style="margin: 0; font-size: 12px; color: #999;">
                    📌 此邮件由 {{.AppName}} 风控系统自动发送<br>
                    🏢 环境: {{.Environment}}<br>
                    🕐 发送时间: {{.Timestamp}}
                </p>
            </div>
        </div>

        <!-- 邮件尾部 -->
        <div class="email-footer">
            <p>© 2025 {{.AppName}}. All rights reserved.</p>
            <p>
                如有疑问，请联系 <a href="mailto:{{.SupportEmail}}">技术支持</a>
            </p>
        </div>
    </div>
</body>
</html>
`

// DefaultBlockEmailTemplate 默认封禁邮件HTML模板
const DefaultBlockEmailTemplate = `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>风控封禁通知</title>
    <style>
        body {
            margin: 0;
            padding: 0;
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background-color: #f5f5f5;
        }
        .email-container {
            max-width: 600px;
            margin: 20px auto;
            background-color: #ffffff;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
        }
        .email-header {
            background: linear-gradient(135deg, #dc3545 0%, #c82333 100%);
            color: #ffffff;
            padding: 30px;
            text-align: center;
        }
        .email-header h1 {
            margin: 0;
            font-size: 28px;
            font-weight: 600;
        }
        .email-header .icon {
            font-size: 48px;
            margin-bottom: 10px;
        }
        .email-body {
            padding: 30px;
        }
        .alert-danger {
            background-color: #f8d7da;
            border-left: 4px solid #dc3545;
            padding: 15px;
            margin: 20px 0;
            border-radius: 4px;
        }
        .alert-danger h2 {
            margin: 0 0 10px 0;
            color: #721c24;
            font-size: 18px;
        }
        .info-table {
            width: 100%;
            border-collapse: collapse;
            margin: 20px 0;
        }
        .info-table th,
        .info-table td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #e0e0e0;
        }
        .info-table th {
            background-color: #f8f9fa;
            font-weight: 600;
            color: #333;
            width: 35%;
        }
        .info-table td {
            color: #666;
        }
        .metric-card {
            display: inline-block;
            width: 48%;
            margin: 1%;
            padding: 15px;
            background-color: #f8f9fa;
            border-radius: 6px;
            text-align: center;
            box-sizing: border-box;
        }
        .metric-card .label {
            font-size: 12px;
            color: #666;
            margin-bottom: 5px;
        }
        .metric-card .value {
            font-size: 24px;
            font-weight: bold;
            color: #dc3545;
        }
        .threshold-bar {
            width: 100%;
            height: 30px;
            background-color: #e0e0e0;
            border-radius: 15px;
            overflow: hidden;
            margin: 20px 0;
        }
        .threshold-fill {
            height: 100%;
            background: linear-gradient(90deg, #dc3545 0%, #c82333 100%);
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-weight: bold;
            font-size: 12px;
        }
        .action-section {
            background-color: #fff3cd;
            border-left: 4px solid #ffc107;
            padding: 15px;
            margin: 20px 0;
            border-radius: 4px;
        }
        .action-section h3 {
            margin: 0 0 10px 0;
            color: #856404;
            font-size: 16px;
        }
        .action-section ul {
            margin: 0;
            padding-left: 20px;
        }
        .action-section li {
            margin: 8px 0;
            color: #666;
        }
        .critical-warning {
            background-color: #fff3cd;
            border: 2px solid #ffc107;
            padding: 20px;
            margin: 20px 0;
            border-radius: 6px;
            text-align: center;
        }
        .critical-warning .warning-icon {
            font-size: 48px;
            margin-bottom: 10px;
        }
        .critical-warning h3 {
            margin: 0 0 10px 0;
            color: #856404;
            font-size: 20px;
        }
        .email-footer {
            background-color: #f8f9fa;
            padding: 20px;
            text-align: center;
            color: #999;
            font-size: 12px;
        }
        .email-footer a {
            color: #2196f3;
            text-decoration: none;
        }
        @media only screen and (max-width: 600px) {
            .metric-card {
                width: 100%;
                margin: 10px 0;
            }
        }
    </style>
</head>
<body>
    <div class="email-container">
        <!-- 邮件头部 -->
        <div class="email-header">
            <div class="icon">🚫</div>
            <h1>风控封禁通知</h1>
            <p style="margin: 10px 0 0 0; opacity: 0.9;">{{.AppName}} - {{.Environment}}</p>
        </div>

        <!-- 邮件正文 -->
        <div class="email-body">
            <!-- 严重警告 -->
            <div class="critical-warning">
                <div class="warning-icon">⚠️</div>
                <h3>用户已被自动封禁</h3>
                <p style="margin: 10px 0 0 0; color: #856404; font-weight: 600;">
                    检测到严重的频率滥用行为，系统已自动执行封禁操作
                </p>
            </div>

            <!-- 封禁信息 -->
            <div class="alert-danger">
                <h2>🚫 封禁详情</h2>
                <p style="margin: 5px 0 0 0; color: #721c24;">
                    用户 <strong>{{.UserID}}</strong> ({{.UserType}}) 因超过最大消息频率阈值被封禁
                </p>
            </div>

            <!-- 详细信息表格 -->
            <table class="info-table">
                <tr>
                    <th>🆔 用户ID</th>
                    <td>{{.UserID}}</td>
                </tr>
                <tr>
                    <th>👤 用户类型</th>
                    <td>{{.UserType}}</td>
                </tr>
                <tr>
                    <th>📊 分钟计数</th>
                    <td><strong style="color: #dc3545;">{{.MinuteCount}}</strong> 条/分钟</td>
                </tr>
                <tr>
                    <th>📊 小时计数</th>
                    <td><strong style="color: #dc3545;">{{.HourCount}}</strong> 条/小时</td>
                </tr>
                <tr>
                    <th>🚫 封禁阈值</th>
                    <td>{{.BlockThreshold}} 条/分钟</td>
                </tr>
                <tr>
                    <th>📈 超出比例</th>
                    <td><strong style="color: #dc3545;">{{.ExceedPercentage}}%</strong></td>
                </tr>
                <tr>
                    <th>🕐 封禁时间</th>
                    <td>{{.Timestamp}}</td>
                </tr>
            </table>

            <!-- 统计指标卡片 -->
            <div style="margin: 20px 0;">
                <div class="metric-card">
                    <div class="label">分钟消息数</div>
                    <div class="value">{{.MinuteCount}}</div>
                </div>
                <div class="metric-card">
                    <div class="label">小时消息数</div>
                    <div class="value">{{.HourCount}}</div>
                </div>
            </div>

            <!-- 阈值进度条 -->
            <div style="margin: 20px 0;">
                <p style="margin: 0 0 8px 0; font-size: 14px; color: #666;">封禁阈值进度（已超限）</p>
                <div class="threshold-bar">
                    <div class="threshold-fill" style="width: 100%;">
                        超限 {{.ExceedPercentage}}%
                    </div>
                </div>
            </div>

            <!-- 紧急处理建议 -->
            <div class="action-section">
                <h3>🔥 紧急处理措施</h3>
                <ul>
                    <li><strong>立即调查</strong>用户 {{.UserID}} 的完整行为日志</li>
                    <li><strong>确认来源</strong>：是机器人攻击还是误封正常用户</li>
                    <li><strong>评估影响</strong>：检查是否还有其他受影响的用户</li>
                    <li><strong>通知相关方</strong>：如确认为攻击，通知安全团队</li>
                    <li><strong>手动解封</strong>：如确认误封，及时手动解除封禁</li>
                    <li><strong>调整策略</strong>：根据实际情况优化风控阈值</li>
                </ul>
            </div>

            <!-- 后续行动 -->
            <div style="background-color: #e8f4fd; border-left: 4px solid #2196f3; padding: 15px; margin: 20px 0; border-radius: 4px;">
                <h3 style="margin: 0 0 10px 0; color: #1976d2; font-size: 16px;">📋 后续跟进</h3>
                <p style="margin: 0; color: #666; line-height: 1.6;">
                    1. 在系统中记录此次封禁事件<br>
                    2. 建立用户行为分析报告<br>
                    3. 评估是否需要升级风控策略<br>
                    4. 定期审查封禁用户列表
                </p>
            </div>

            <!-- 系统信息 -->
            <div style="margin-top: 30px; padding-top: 20px; border-top: 1px solid #e0e0e0;">
                <p style="margin: 0; font-size: 12px; color: #999;">
                    📌 此邮件由 {{.AppName}} 风控系统自动发送<br>
                    🏢 环境: {{.Environment}}<br>
                    🕐 封禁时间: {{.Timestamp}}<br>
                    ⚠️ 优先级: <strong style="color: #dc3545;">紧急</strong>
                </p>
            </div>
        </div>

        <!-- 邮件尾部 -->
        <div class="email-footer">
            <p>© 2025 {{.AppName}}. All rights reserved.</p>
            <p>
                紧急联系: <a href="mailto:{{.SecurityEmail}}">安全团队</a> | 
                技术支持: <a href="mailto:{{.SupportEmail}}">技术支持</a>
            </p>
        </div>
    </div>
</body>
</html>
`
