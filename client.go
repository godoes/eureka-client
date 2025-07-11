package eureka_client

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Client eureka客户端
type Client struct {
	logger Logger

	// for monitor system signal
	signalChan chan os.Signal
	mutex      sync.RWMutex
	running    bool

	Config   *Config
	Instance *Instance

	// eureka服务中注册的应用
	Applications *Applications
}

// Option 自定义
type Option func(instance *Instance)

// SetLogger 设置日志实现
func (c *Client) SetLogger(logger Logger) {
	c.logger = logger
}

// Start 启动时注册客户端，并后台刷新服务列表，以及心跳
func (c *Client) Start() {
	c.mutex.Lock()
	c.running = true
	c.mutex.Unlock()
	// 刷新服务列表
	go c.refresh()
	// 心跳
	go c.heartbeat()
	// 监听退出信号，自动删除注册信息
	go c.handleSignal()
}

// refresh 刷新服务列表
func (c *Client) refresh() {
	timer := time.NewTimer(0)
	defer timer.Stop()

	interval := time.Duration(c.Config.RegistryFetchIntervalSeconds) * time.Second
	for c.running {
		<-timer.C

		if err := c.doRefresh(); err != nil {
			c.logger.Error("refresh application instance failed", err)
		} else {
			c.logger.Debug("refresh application instance successful")
		}

		// reset interval
		timer.Reset(interval)
	}
}

// ConnectDetection 连接检测
func (c *Client) ConnectDetection() error {
	err := c.doHeartbeat()
	if err == nil {
		c.logger.Debug("heartbeat application instance successful")
		return nil
	} else if errors.Is(err, ErrNotFound) {
		// heartbeat not found, need register
		return nil
	} else {
		c.logger.Error("heartbeat application instance failed", err)
		return err
	}
}

// heartbeat 心跳
func (c *Client) heartbeat() {
	timer := time.NewTimer(0)
	defer timer.Stop()

	interval := time.Duration(c.Config.RenewalIntervalInSecs) * time.Second
	for c.running {
		<-timer.C

		err := c.doHeartbeat()
		if err == nil {
			c.logger.Debug("heartbeat application instance successful")
		} else if errors.Is(err, ErrNotFound) {
			// heartbeat not found, need register
			err = c.doRegister()
			if err == nil {
				c.logger.Info("register application instance successful")
			} else {
				c.logger.Error("register application instance failed", err)
			}
		} else {
			c.logger.Error("heartbeat application instance failed", err)
		}

		// reset interval
		timer.Reset(interval)
	}
}

func (c *Client) doRegister() error {
	return Register(c.Config.DefaultZone, c.Config.App, c.Instance)
}

func (c *Client) doUnRegister() error {
	return UnRegister(c.Config.DefaultZone, c.Instance.App, c.Instance)
}

func (c *Client) doHeartbeat() error {
	return Heartbeat(c.Config.DefaultZone, c.Instance.App, c.Instance.InstanceID)
}

func (c *Client) doRefresh() error {
	// todo If the delta is disabled or if it is the first time, get all applications

	// get all applications
	applications, err := Refresh(c.Config.DefaultZone)
	if err != nil {
		return err
	}

	// set applications
	c.mutex.Lock()
	c.Applications = applications
	c.mutex.Unlock()
	return nil
}

// handleSignal 监听退出信号，删除注册的实例
func (c *Client) handleSignal() {
	if c.signalChan == nil {
		c.signalChan = make(chan os.Signal, 1)
	}
	signal.Notify(c.signalChan, syscall.SIGTERM, syscall.SIGINT)
	defer func() {
		signal.Stop(c.signalChan)
		close(c.signalChan)
	}()

	for c.running {
		switch <-c.signalChan {
		case syscall.SIGINT, syscall.SIGTERM:
			c.logger.Info("receive exit signal, client instance going to de-register")
			err := c.doUnRegister()
			if err != nil {
				c.logger.Error("de-register application instance failed", err)
			} else {
				c.logger.Info("de-register application instance successful")
			}
			os.Exit(0)
		}
	}
}

// NewClient 创建客户端
func NewClient(config *Config, opts ...Option) *Client {
	DefaultConfig(config)
	instance := NewInstance(config)
	client := &Client{
		logger:   NewLogger(config.LogLevel),
		Config:   config,
		Instance: instance,
	}
	for _, opt := range opts {
		opt(client.Instance)
	}
	return client
}

func DefaultConfig(config *Config) {
	if config.DefaultZone == "" {
		config.DefaultZone = "http://localhost:8761/eureka/"
	}
	if !strings.HasSuffix(config.DefaultZone, "/") {
		config.DefaultZone = config.DefaultZone + "/"
	}
	if config.RenewalIntervalInSecs == 0 {
		config.RenewalIntervalInSecs = 30
	}
	if config.RegistryFetchIntervalSeconds == 0 {
		config.RegistryFetchIntervalSeconds = 15
	}
	if config.DurationInSecs == 0 {
		config.DurationInSecs = 90
	}
	if config.App == "" {
		config.App = "unknown"
	} else {
		config.App = strings.ToLower(config.App)
	}
	if config.IP == "" {
		config.IP = GetLocalIP()
	}
	if config.HostName == "" {
		config.HostName = config.IP
	}
	if config.Port == 0 {
		config.Port = 80
	}
	if config.InstanceID == "" {
		config.InstanceID = fmt.Sprintf("%s:%s:%d", config.App, config.IP, config.Port)
	}
}

// GetApplicationInstance 根据服务名获取注册的服务实例列表
func (c *Client) GetApplicationInstance(name string) []Instance {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var instances []Instance
	if c.Applications != nil {
		for _, app := range c.Applications.Applications {
			if app.Name == name && app.Instances != nil {
				instances = append(instances, app.Instances...)
			}
		}
	}
	return instances
}
