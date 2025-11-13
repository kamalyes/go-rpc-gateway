// 脱敏规则使用示例
package pbmo

import (
	"fmt"
	"strings"
)

// ExampleUsage 展示如何使用灵活的脱敏规则系统
func ExampleUsage() {
	// 创建转换器示例
	var pb SomeProtoBuf
	var model SomeModel

	converter := NewAdvancedConverter(&pb, &model,
		WithDesensitization(true, true),
	)

	// 1. 注册自定义脱敏类型
	converter.RegisterDesensitizationType("sensitive", "custom")
	converter.RegisterDesensitizationType("secret", "password")
	converter.RegisterDesensitizationType("mobile", "phoneNumber")

	// 2. 注册自定义解析器 - 支持范围脱敏
	converter.RegisterCustomParser("range", func(tag string, rule *DesensitizeRule) error {
		// 解析 range:1-3 格式
		if strings.HasPrefix(tag, "range:") {
			rangeStr := strings.TrimPrefix(tag, "range:")
			parts := strings.Split(rangeStr, "-")
			if len(parts) == 2 {
				// 解析起始和结束位置
				// 这里可以添加具体的解析逻辑
				rule.Type = "range"
				rule.Config = map[string]string{
					"start": parts[0],
					"end":   parts[1],
				}
			}
		}
		return nil
	})

	// 3. 注册正则表达式解析器
	converter.RegisterCustomParser("regex", func(tag string, rule *DesensitizeRule) error {
		// 解析 regex:pattern 格式
		if strings.HasPrefix(tag, "regex:") {
			pattern := strings.TrimPrefix(tag, "regex:")
			rule.Type = "regex"
			rule.Config = map[string]string{
				"pattern": pattern,
			}
		}
		return nil
	})

	// 4. 注册百分比脱敏解析器
	converter.RegisterCustomParser("percent", func(tag string, rule *DesensitizeRule) error {
		// 解析 percent:50 格式 (脱敏50%的字符)
		if strings.HasPrefix(tag, "percent:") {
			percentStr := strings.TrimPrefix(tag, "percent:")
			rule.Type = "percent"
			rule.Config = map[string]string{
				"percent": percentStr,
			}
		}
		return nil
	})

	fmt.Println("脱敏规则注册完成")

	// 查看当前的类型映射
	mappings := converter.GetDesensitizationTypeMapping()
	fmt.Printf("当前类型映射: %+v\n", mappings)

	// 查看性能信息
	perfInfo := converter.GetPerformanceInfo()
	fmt.Printf("性能信息: %+v\n", perfInfo)
}

// SomeModel 示例模型，展示各种脱敏标签
type SomeModel struct {
	Email        string `desensitize:"email"`              // 标准邮箱脱敏
	Phone        string `desensitize:"phone"`              // 标准手机脱敏
	BankCard     string `desensitize:"bankCard"`           // 标准银行卡脱敏
	CustomField  string `desensitize:"custom:mask(2,6,*)"` // 自定义掩码脱敏
	RangeField   string `desensitize:"range:1-3"`          // 自定义范围脱敏
	RegexField   string `desensitize:"regex:[a-z]+"`       // 自定义正则脱敏
	PercentField string `desensitize:"percent:60"`         // 自定义百分比脱敏
	Sensitive    string `desensitize:"sensitive"`          // 注册的自定义类型
}

// SomeProtoBuf 示例PB结构
type SomeProtoBuf struct {
	Email        string
	Phone        string
	BankCard     string
	CustomField  string
	RangeField   string
	RegexField   string
	PercentField string
	Sensitive    string
}

// DynamicDesensitizationExample 动态脱敏规则示例
func DynamicDesensitizationExample() {
	var pb SomeProtoBuf
	var model SomeModel

	converter := NewAdvancedConverter(&pb, &model,
		WithDesensitization(true, true),
	)

	// 运行时动态注册新的脱敏类型
	converter.RegisterDesensitizationType("businessId", "custom")
	converter.RegisterDesensitizationType("socialId", "identityCard")

	// 动态注册复杂的自定义解析器
	converter.RegisterCustomParser("complex", func(tag string, rule *DesensitizeRule) error {
		// 解析复杂格式: complex:type=mask;start=1;end=5;char=*;extra=value
		if strings.HasPrefix(tag, "complex:") {
			configStr := strings.TrimPrefix(tag, "complex:")
			pairs := strings.Split(configStr, ";")

			config := make(map[string]string)
			for _, pair := range pairs {
				kv := strings.Split(pair, "=")
				if len(kv) == 2 {
					config[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				}
			}

			rule.Type = "complex"
			rule.Config = config
		}
		return nil
	})

	fmt.Println("动态脱敏规则配置完成，系统现在支持:")
	fmt.Println("- 运行时注册新的脱敏类型")
	fmt.Println("- 复杂的参数化脱敏规则")
	fmt.Println("- 可扩展的解析器系统")
	fmt.Println("- 完全可配置的脱敏策略")
}
