# 🌐 CoreDNS Multi-Cluster Configuration Manager

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个用于管理多个 Kubernetes 集群 CoreDNS 配置的 Web 应用。适用于 BGP 网络环境下跨集群服务发现场景，无需部署 Service Mesh。

## ✨ 功能特性

- 🔐 **简单认证** - 用户名/密码登录 + JWT Token
- 📦 **多集群管理** - 添加/删除多个 K8s 集群
- 👁️ **CoreDNS 查看** - 查看 ConfigMap 和 Service 信息
- ⚡ **快速配置** - 一键添加 namespace 转发规则
- ✏️ **在线编辑** - 直接编辑 Corefile 并保存

## 🚀 快速开始

### 使用 Docker
```bash
# 直接运行
docker run -d -p 80:80 \
  -v $(pwd)/data:/app/data \
  -e AUTH_USERNAME=admin \
  -e AUTH_PASSWORD=admin123 \
  -e AUTH_JWT_SECRET=coredns-manager-secret-key-change-me \
  yamabuki/coredns-manager:latest
```


### 本地运行

```bash
# 安装 templ CLI
go install github.com/a-h/templ/cmd/templ@latest

# 生成模板代码
templ generate

# 运行
go run main.go
```

访问 `http://localhost:80` - 默认账号: `admin` / `admin`

## 📖 使用说明

### 添加集群

1. 点击 **添加集群** 按钮
2. 输入集群名称
3. 粘贴 kubeconfig 内容
4. 点击添加（自动验证连接）

### 添加转发规则

1. 点击集群卡片进入 CoreDNS 配置
2. 切换到 **转发规则** 标签
3. 输入 `namespace` 和目标 DNS IP
4. 自动生成格式：

```
namespace.svc.cluster.local:53 {
    forward . 10.96.0.10
}
```

## ⚙️ 配置

编辑 `config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

auth:
  username: "admin"
  password: "your-password"
  jwt_secret: "your-secret-key"

data_dir: "./data"
```

## 📁 项目结构

```
coredns-multi-configuration/
├── main.go                 # 入口
├── config.yaml             # 配置文件
├── Dockerfile              # Docker 构建
├── pkg/
│   ├── models/             # 数据模型
│   ├── store/              # JSON 存储
│   ├── k8s/                # K8s 客户端
│   ├── auth/               # JWT 认证
│   └── handlers/           # HTTP 处理器
└── templates/              # Templ 模板
```

## 🔧 技术栈

- **后端**: Go + Gin
- **前端**: Templ + HTMX
- **存储**: 本地 JSON 文件
- **K8s**: client-go

## 📄 License

MIT License
