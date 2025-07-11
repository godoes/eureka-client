module github.com/godoes/eureka-client

go 1.18

require golang.org/x/sync v0.11.0

retract v0.3.0 // 由于序列化 eureka 客户端配置信息时未忽略 LogLevel 字段而导致注册失败
