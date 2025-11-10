package config

// PProfConfig 性能分析配置
type PProfConfig struct {
	Enabled     bool                   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	PathPrefix  string                 `mapstructure:"path_prefix" yaml:"path_prefix" json:"path_prefix"`
	Auth        PProfAuthConfig        `mapstructure:"auth" yaml:"auth" json:"auth"`
	Scenarios   PProfScenariosConfig   `mapstructure:"scenarios" yaml:"scenarios" json:"scenarios"`
	AutoCollect PProfAutoCollectConfig `mapstructure:"auto_collect" yaml:"auto_collect" json:"auto_collect"`
}

// PProfAuthConfig pprof 认证配置
type PProfAuthConfig struct {
	Enabled    bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Token      string   `mapstructure:"token" yaml:"token" json:"token"`
	AllowedIPs []string `mapstructure:"allowed_ips" yaml:"allowed_ips" json:"allowed_ips"`
}

// PProfScenariosConfig pprof 场景配置
type PProfScenariosConfig struct {
	CPUProfile  bool `mapstructure:"cpu_profile" yaml:"cpu_profile" json:"cpu_profile"`
	HeapProfile bool `mapstructure:"heap_profile" yaml:"heap_profile" json:"heap_profile"`
	Goroutine   bool `mapstructure:"goroutine" yaml:"goroutine" json:"goroutine"`
	Block       bool `mapstructure:"block" yaml:"block" json:"block"`
	Mutex       bool `mapstructure:"mutex" yaml:"mutex" json:"mutex"`
	Trace       bool `mapstructure:"trace" yaml:"trace" json:"trace"`
}

// PProfAutoCollectConfig pprof 自动采集配置
type PProfAutoCollectConfig struct {
	Enabled     bool   `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Interval    string `mapstructure:"interval" yaml:"interval" json:"interval"`
	CPUDuration string `mapstructure:"cpu_duration" yaml:"cpu_duration" json:"cpu_duration"`
	OutputDir   string `mapstructure:"output_dir" yaml:"output_dir" json:"output_dir"`
}
