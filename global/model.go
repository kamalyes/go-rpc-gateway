/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2023-07-28 00:50:58
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2025-11-12 15:30:49
 * @FilePath: \go-rpc-gateway\global\model.go
 * @Description: 增强的数据库模型基础结构，支持企业级应用需求
 *
 * Copyright (c) 2024 by kamalyes, All Rights Reserved.
 */

package global

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kamalyes/go-rpc-gateway/errors"
	"gorm.io/gorm"
)

type DistributedId int64
type TTime time.Time

// Model 增强的基础模型，包含企业级应用常用字段
type Model struct {
	ID         DistributedId  `json:"id,omitempty"            gorm:"column:id;primary_key;comment:主键ID"`
	CreateTime TTime          `json:"createTime,omitempty"    gorm:"column:create_time;comment:创建时间;index"`
	UpdateTime TTime          `json:"updateTime,omitempty"    gorm:"column:update_time;comment:更新时间;index"`
	DeleteTime gorm.DeletedAt `json:"deleteTime,omitempty"    gorm:"column:delete_time;comment:删除时间;index"`
	CreateBy   DistributedId  `json:"createBy,omitempty"      gorm:"column:create_by;comment:创建者ID;index"`
	UpdateBy   DistributedId  `json:"updateBy,omitempty"      gorm:"column:update_by;comment:更新者ID;index"`
	Version    int32          `json:"version,omitempty"       gorm:"column:version;default:1;comment:乐观锁版本号"`
	Status     int8           `json:"status,omitempty"        gorm:"column:status;default:1;comment:状态(1:正常 0:禁用);index"`
	Remark     string         `json:"remark,omitempty"        gorm:"column:remark;type:varchar(1000);comment:备注"`
}

// StatusEnum 状态枚举
const (
	StatusDisabled = 0 // 禁用
	StatusEnabled  = 1 // 启用
)

// TenantModel 多租户模型，继承基础模型
type TenantModel struct {
	Model
	TenantID DistributedId `json:"tenantId,omitempty" gorm:"column:tenant_id;comment:租户ID;index"`
}

// 商户模型，继承基础模型
type MerchantModel struct {
	Model
	MerchantID DistributedId `json:"merchantId,omitempty" gorm:"column:merchant_id;comment:商户ID;index"`
}

// 商户&门店模型，继承基础模型
type MerchantShopModel struct {
	Model
	MerchantID DistributedId `json:"merchantId,omitempty" gorm:"column:merchant_id;comment:商户ID;index"`
	ShopID     DistributedId `json:"shopId,omitempty"     gorm:"column:shop_id;comment:店铺ID;index"`
}

// SoftDeleteModel 软删除模型（兼容旧版本）
type SoftDeleteModel struct {
	ID         DistributedId  `json:"id,omitempty"         gorm:"column:id;primary_key;comment:主键ID"`
	CreateTime TTime          `json:"createTime,omitempty" gorm:"column:create_time;comment:创建时间;index"`
	UpdateTime TTime          `json:"updateTime,omitempty" gorm:"column:update_time;comment:更新时间;index"`
	DeleteTime gorm.DeletedAt `json:"deleteTime,omitempty" gorm:"column:delete_time;comment:删除时间;index"`
}

// CreateId 创建一个分布式ID（雪花ID）
func CreateId() DistributedId {
	if Node == nil {
		// 如果雪花ID节点未初始化，返回时间戳作为fallback
		return DistributedId(time.Now().UnixNano())
	}
	id := Node.Generate()
	return DistributedId(id.Int64())
}

// CreateTime 创建当前时间的 TTime 实例
func CreateTime() TTime {
	return TTime(time.Now())
}

// TTime 自定义时间类型的方法实现

// MarshalJSON 实现 JSON 序列化
func (t TTime) MarshalJSON() ([]byte, error) {
	if time.Time(t).IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(time.Time(t).Format("2006-01-02 15:04:05"))
}

// UnmarshalJSON 实现 JSON 反序列化
func (t *TTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var timeStr string
	if err := json.Unmarshal(data, &timeStr); err != nil {
		return err
	}
	parsedTime, err := time.Parse("2006-01-02 15:04:05", timeStr)
	if err != nil {
		return err
	}
	*t = TTime(parsedTime)
	return nil
}

// Value 实现 driver.Valuer 接口，用于数据库存储
func (t TTime) Value() (driver.Value, error) {
	if time.Time(t).IsZero() {
		return nil, nil
	}
	return time.Time(t), nil
}

// Scan 实现 sql.Scanner 接口，用于数据库读取
func (t *TTime) Scan(value interface{}) error {
	if value == nil {
		*t = TTime(time.Time{})
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*t = TTime(v)
	case string:
		parsedTime, err := time.Parse("2006-01-02 15:04:05", v)
		if err != nil {
			return err
		}
		*t = TTime(parsedTime)
	default:
		return errors.NewErrorf(errors.ErrCodeScanTypeMismatch, "cannot scan %T into TTime", value)
	}
	return nil
}

// String 实现 Stringer 接口
func (t TTime) String() string {
	if time.Time(t).IsZero() {
		return ""
	}
	return time.Time(t).Format("2006-01-02 15:04:05")
}

// ToTime 转换为标准 time.Time
func (t TTime) ToTime() time.Time {
	return time.Time(t)
}

// IsZero 检查时间是否为零值
func (t TTime) IsZero() bool {
	return time.Time(t).IsZero()
}

// DistributedId 分布式ID的方法实现

// String 实现 Stringer 接口
func (id DistributedId) String() string {
	return fmt.Sprintf("%d", int64(id))
}

// IsZero 检查ID是否为零值
func (id DistributedId) IsZero() bool {
	return id == 0
}

// ToInt64 转换为 int64
func (id DistributedId) ToInt64() int64 {
	return int64(id)
}

// Model 的增强方法

// BeforeCreate GORM 钩子：创建前自动填充字段
func (m *Model) BeforeCreate(tx *gorm.DB) error {
	if m.ID.IsZero() {
		m.ID = CreateId()
	}
	now := CreateTime()
	if m.CreateTime.IsZero() {
		m.CreateTime = now
	}
	if m.UpdateTime.IsZero() {
		m.UpdateTime = now
	}
	if m.Version == 0 {
		m.Version = 1
	}
	if m.Status == 0 {
		m.Status = StatusEnabled
	}
	return nil
}

// BeforeUpdate GORM 钩子：更新前自动更新字段
func (m *Model) BeforeUpdate(tx *gorm.DB) error {
	m.UpdateTime = CreateTime()
	// 乐观锁：版本号自增
	if tx.Statement.Changed("version") {
		m.Version++
	}
	return nil
}

// IsEnabled 检查状态是否为启用
func (m *Model) IsEnabled() bool {
	return m.Status == StatusEnabled
}

// IsDisabled 检查状态是否为禁用
func (m *Model) IsDisabled() bool {
	return m.Status == StatusDisabled
}

// Enable 启用状态
func (m *Model) Enable() {
	m.Status = StatusEnabled
	m.UpdateTime = CreateTime()
}

// Disable 禁用状态
func (m *Model) Disable() {
	m.Status = StatusDisabled
	m.UpdateTime = CreateTime()
}

// SetCreateBy 设置创建者
func (m *Model) SetCreateBy(userID DistributedId) {
	m.CreateBy = userID
}

// SetUpdateBy 设置更新者
func (m *Model) SetUpdateBy(userID DistributedId) {
	m.UpdateBy = userID
	m.UpdateTime = CreateTime()
}

// SetRemark 设置备注
func (m *Model) SetRemark(remark string) {
	m.Remark = remark
	m.UpdateTime = CreateTime()
}

// GetCreateTime 获取创建时间
func (m *Model) GetCreateTime() time.Time {
	return m.CreateTime.ToTime()
}

// GetUpdateTime 获取更新时间
func (m *Model) GetUpdateTime() time.Time {
	return m.UpdateTime.ToTime()
}

// TenantModel 的增强方法

// BeforeCreate 多租户模型创建前钩子
func (tm *TenantModel) BeforeCreate(tx *gorm.DB) error {
	// 先调用基础模型的钩子
	if err := tm.Model.BeforeCreate(tx); err != nil {
		return err
	}
	// 多租户特定逻辑可以在这里添加
	return nil
}

// SetTenantID 设置租户ID
func (tm *TenantModel) SetTenantID(tenantID DistributedId) {
	tm.TenantID = tenantID
}

// GetTenantID 获取租户ID
func (tm *TenantModel) GetTenantID() DistributedId {
	return tm.TenantID
}
