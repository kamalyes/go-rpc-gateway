package pbmo

import (
	"github.com/go-playground/validator/v10"
)

// PBValidator 全局PB结构体验证器
var PBValidator *validator.Validate

// 初始化验证器
func init() {
	PBValidator = validator.New()
}

// ValidateStruct 验证任意结构体
func ValidateStruct(s interface{}) error {
	return PBValidator.Struct(s)
}
