package eureka_client

import (
	"fmt"
)

// Config eureka 客户端配置
type Config struct {
	// eureka 服务端地址
	DefaultZone string
	// 心跳间隔，默认 30s
	RenewalIntervalInSecs int
	// 获取服务列表间隔，默认 15s
	RegistryFetchIntervalSeconds int
	// 过期间隔，默认 90s
	DurationInSecs int
	// 实例ID，默认 app:ip:port
	InstanceID string
	// 应用名称
	App string
	// 应用版本
	Version string
	// Host，为空则取 IP
	HostName string
	// IP，为空则取本地 IP
	IP string
	// 端口，默认 80
	Port int
	// 元数据
	Metadata map[string]interface{}
	// 自定义日志级别: DEBUG / INFO / WARN / ERROR
	//  - 非 eureka 客户端标准配置，不能序列化到请求内容中
	LogLevel int `json:"-" xml:"-"`
}

// Applications eureka 服务端注册的 apps
type Applications struct {
	VersionsDelta string        `xml:"versions__delta,omitempty" json:"versions__delta,omitempty"`
	AppsHashcode  string        `xml:"apps__hashcode,omitempty" json:"apps__hashcode,omitempty"`
	Applications  []Application `xml:"application,omitempty" json:"application,omitempty"`
}

// Application eureka 服务端注册的 app
type Application struct {
	Name      string     `xml:"name" json:"name"`
	Instances []Instance `xml:"instance" json:"instance"`
}

// Instance 服务实例
type Instance struct {
	HostName                      string                 `xml:"hostName" json:"hostName"`
	HomePageURL                   string                 `xml:"homePageUrl,omitempty" json:"homePageUrl,omitempty"`
	StatusPageURL                 string                 `xml:"statusPageUrl" json:"statusPageUrl"`
	HealthCheckURL                string                 `xml:"healthCheckUrl,omitempty" json:"healthCheckUrl,omitempty"`
	App                           string                 `xml:"app" json:"app"`
	Version                       string                 `xml:"version" json:"version"`
	IPAddr                        string                 `xml:"ipAddr" json:"ipAddr"`
	VipAddress                    string                 `xml:"vipAddress" json:"vipAddress"`
	SecureVipAddress              string                 `xml:"secureVipAddress,omitempty" json:"secureVipAddress,omitempty"`
	Status                        string                 `xml:"status" json:"status"`
	Port                          *Port                  `xml:"port,omitempty" json:"port,omitempty"`
	SecurePort                    *Port                  `xml:"securePort,omitempty" json:"securePort,omitempty"`
	DataCenterInfo                *DataCenterInfo        `xml:"dataCenterInfo" json:"dataCenterInfo"`
	LeaseInfo                     *LeaseInfo             `xml:"leaseInfo,omitempty" json:"leaseInfo,omitempty"`
	Metadata                      map[string]interface{} `xml:"metadata,omitempty" json:"metadata,omitempty"`
	IsCoordinatingDiscoveryServer string                 `xml:"isCoordinatingDiscoveryServer,omitempty" json:"isCoordinatingDiscoveryServer,omitempty"`
	LastUpdatedTimestamp          string                 `xml:"lastUpdatedTimestamp,omitempty" json:"lastUpdatedTimestamp,omitempty"`
	LastDirtyTimestamp            string                 `xml:"lastDirtyTimestamp,omitempty" json:"lastDirtyTimestamp,omitempty"`
	ActionType                    string                 `xml:"actionType,omitempty" json:"actionType,omitempty"`
	OverriddenStatus              string                 `xml:"overriddenstatus,omitempty" json:"overriddenstatus,omitempty"`
	CountryID                     int                    `xml:"countryId,omitempty" json:"countryId,omitempty"`
	InstanceID                    string                 `xml:"instanceId,omitempty" json:"instanceId,omitempty"`

	EurekaConfig *Config
	Beater       *BeatReactor
}

// Port 端口
type Port struct {
	Port    int    `xml:",chardata" json:"$"`
	Enabled string `xml:"enabled,attr" json:"@enabled"`
}

// DataCenterInfo 数据中心信息
type DataCenterInfo struct {
	Name     string              `xml:"name" json:"name"`
	Class    string              `xml:"class,attr" json:"@class"`
	Metadata *DataCenterMetadata `xml:"metadata,omitempty" json:"metadata,omitempty"`
}

// DataCenterMetadata 数据中心信息元数据
type DataCenterMetadata struct {
	AmiLaunchIndex   string `xml:"ami-launch-index,omitempty" json:"ami-launch-index,omitempty"`
	LocalHostname    string `xml:"local-hostname,omitempty" json:"local-hostname,omitempty"`
	AvailabilityZone string `xml:"availability-zone,omitempty" json:"availability-zone,omitempty"`
	InstanceID       string `xml:"instance-id,omitempty" json:"instance-id,omitempty"`
	PublicIpv4       string `xml:"public-ipv4,omitempty" json:"public-ipv4,omitempty"`
	PublicHostname   string `xml:"public-hostname,omitempty" json:"public-hostname,omitempty"`
	AmiManifestPath  string `xml:"ami-manifest-path,omitempty" json:"ami-manifest-path,omitempty"`
	LocalIpv4        string `xml:"local-ipv4,omitempty" json:"local-ipv4,omitempty"`
	Hostname         string `xml:"hostname,omitempty" json:"hostname,omitempty"`
	AmiID            string `xml:"ami-id,omitempty" json:"ami-id,omitempty"`
	InstanceType     string `xml:"instance-type,omitempty" json:"instance-type,omitempty"`
}

// LeaseInfo 续约信息
type LeaseInfo struct {
	RenewalIntervalInSecs int `xml:"renewalIntervalInSecs,omitempty" json:"renewalIntervalInSecs,omitempty"`
	DurationInSecs        int `xml:"durationInSecs,omitempty" json:"durationInSecs,omitempty"`
}

// NewInstance 创建服务实例
func NewInstance(config *Config) *Instance {
	instance := &Instance{
		InstanceID: config.InstanceID,
		HostName:   config.HostName,
		App:        config.App,
		Version:    config.Version,
		IPAddr:     config.IP,
		Port: &Port{
			Port:    config.Port,
			Enabled: "true",
		},
		VipAddress:       config.App,
		SecureVipAddress: config.App,
		// 续约信息
		LeaseInfo: &LeaseInfo{
			RenewalIntervalInSecs: config.RenewalIntervalInSecs,
			DurationInSecs:        config.DurationInSecs,
		},
		Status:           "UP",
		OverriddenStatus: "UNKNOWN",
		// 数据中心
		DataCenterInfo: &DataCenterInfo{
			Name:  "MyOwn",
			Class: "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
		},
		// 元数据
		Metadata: config.Metadata,
	}
	instance.HomePageURL = fmt.Sprintf("%s://%s:%d", "http", config.IP, config.Port)
	instance.StatusPageURL = fmt.Sprintf("%s://%s:%d/info", "http", config.IP, config.Port)
	instance.HealthCheckURL = fmt.Sprintf("%s://%s:%d/health", "http", config.IP, config.Port)
	instance.EurekaConfig = config
	beater := NewBeatReactor(config, int64(config.RenewalIntervalInSecs))
	instance.Beater = &beater
	return instance
}
