package pbmo

import (
	"time"
)

// Go模型结构体，对应proto定义

type GoStatus int

const (
	GoStatusUnknown  GoStatus = 0
	GoStatusActive   GoStatus = 1
	GoStatusInactive GoStatus = 2
	GoStatusPending  GoStatus = 3
)

type GoPriority int

const (
	GoPriorityLow      GoPriority = 0
	GoPriorityMedium   GoPriority = 1
	GoPriorityHigh     GoPriority = 2
	GoPriorityCritical GoPriority = 3
)

// 地址结构体
type GoAddress struct {
	Street     string `json:"street" gorm:"column:street" validate:"required"`
	City       string `json:"city" gorm:"column:city" validate:"required"`
	Country    string `json:"country" gorm:"column:country" validate:"required"`
	PostalCode string `json:"postal_code" gorm:"column:postal_code" validate:"required"`
}

// 联系方式结构体
type GoContact struct {
	Email   string `json:"email" gorm:"column:email" validate:"email"`
	Phone   string `json:"phone" gorm:"column:phone" validate:"required"`
	Website string `json:"website" gorm:"column:website"`
}

// 用户结构体 - 包含所有基础类型
type GoUser struct {
	// 数值类型
	ID      int64   `json:"id" gorm:"primaryKey;column:id"`
	Age     int32   `json:"age" gorm:"column:age" validate:"min=0,max=150"`
	Score   uint32  `json:"score" gorm:"column:score"`
	Balance uint64  `json:"balance" gorm:"column:balance;type:decimal(10,2)"`
	Rating  float32 `json:"rating" gorm:"column:rating"`
	Salary  float64 `json:"salary" gorm:"column:salary;type:decimal(15,2)"`

	// 字符串类型
	Name     string `json:"name" gorm:"column:name" validate:"required,min=2,max=50"`
	Username string `json:"username" gorm:"column:username;unique" validate:"required,alphanum"`
	Bio      string `json:"bio" gorm:"column:bio;type:text"`

	// 布尔类型
	IsActive   bool `json:"is_active" gorm:"column:is_active;default:true"`
	IsVerified bool `json:"is_verified" gorm:"column:is_verified;default:false"`

	// 字节类型
	Avatar    []byte `json:"avatar" gorm:"column:avatar;type:blob"`
	Signature []byte `json:"signature" gorm:"column:signature"`

	// 枚举类型
	Status   GoStatus   `json:"status" gorm:"column:status;type:int" validate:"required"`
	Priority GoPriority `json:"priority" gorm:"column:priority;type:int"`

	// 嵌套结构体
	Address GoAddress `json:"address" gorm:"embedded;embeddedPrefix:address_"`
	Contact GoContact `json:"contact" gorm:"embedded;embeddedPrefix:contact_"`

	// 切片类型
	Tags        []string `json:"tags" gorm:"type:json"`
	Scores      []int32  `json:"scores" gorm:"type:json"`
	Preferences []bool   `json:"preferences" gorm:"type:json"`

	// 可选字段（使用指针）
	Nickname  *string `json:"nickname,omitempty" gorm:"column:nickname"`
	MiddleAge *int32  `json:"middle_age,omitempty" gorm:"column:middle_age"`
	IsPremium *bool   `json:"is_premium,omitempty" gorm:"column:is_premium"`

	// Map类型
	Metadata map[string]string `json:"metadata" gorm:"type:json"`
	Settings map[string]int32  `json:"settings" gorm:"type:json"`
	Flags    map[string]bool   `json:"flags" gorm:"type:json"`

	// 时间字段（Go模型中添加）
	CreatedAt time.Time  `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"column:updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"column:deleted_at"`
}

// 产品结构体
type GoProduct struct {
	ID          uint64            `json:"id" gorm:"primaryKey;column:id"`
	Name        string            `json:"name" gorm:"column:name" validate:"required"`
	Price       float64           `json:"price" gorm:"column:price;type:decimal(10,2)" validate:"min=0"`
	InStock     bool              `json:"in_stock" gorm:"column:in_stock;default:true"`
	Category    GoStatus          `json:"category" gorm:"column:category;type:int"`
	Images      []string          `json:"images" gorm:"type:json"`
	Properties  map[string]string `json:"properties" gorm:"type:json"`
	Description *string           `json:"description,omitempty" gorm:"column:description;type:text"`
	CreatedAt   time.Time         `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time         `json:"updated_at" gorm:"column:updated_at"`
}

// 订单结构体
type GoOrder struct {
	ID              int64             `json:"id" gorm:"primaryKey;column:id"`
	UserID          int64             `json:"user_id" gorm:"column:user_id" validate:"required"`
	Total           float64           `json:"total" gorm:"column:total;type:decimal(10,2)"`
	Status          GoStatus          `json:"status" gorm:"column:status;type:int"`
	Items           []GoProduct       `json:"items" gorm:"type:json"`
	ShippingAddress GoAddress         `json:"shipping_address" gorm:"embedded;embeddedPrefix:shipping_"`
	Notes           map[string]string `json:"notes" gorm:"type:json"`
	CreatedAt       time.Time         `json:"created_at" gorm:"column:created_at"`
	UpdatedAt       time.Time         `json:"updated_at" gorm:"column:updated_at"`
}

// 响应结构体
type GoUserResponse struct {
	Success    bool     `json:"success"`
	Message    string   `json:"message"`
	User       *GoUser  `json:"user,omitempty"`
	Users      []GoUser `json:"users"`
	TotalCount int32    `json:"total_count"`
}

// 请求结构体
type GoGetUserRequest struct {
	ID              int64 `json:"id" validate:"required,min=1"`
	IncludeInactive *bool `json:"include_inactive,omitempty"`
}

type GoListUsersRequest struct {
	Page     int32             `json:"page" validate:"min=1"`
	PageSize int32             `json:"page_size" validate:"min=1,max=100"`
	Status   *GoStatus         `json:"status,omitempty"`
	Search   *string           `json:"search,omitempty"`
	Filters  map[string]string `json:"filters"`
}
