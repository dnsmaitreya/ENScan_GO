# 自动登录功能说明

## 功能概述

ENScan_GO 现已支持爱企查(AQC)的自动登录功能，当Cookie为空或失效时，程序可以自动使用账号密码登录并更新Cookie。

## 功能特性

- ✅ 自动检测Cookie是否失效
- ✅ Cookie失效时自动重新登录
- ✅ 登录成功后自动更新配置文件
- ✅ 支持Docker环境（需要安装Chromium）
- ✅ 完全向后兼容，不影响手动Cookie配置方式

## 配置方法

### 1. 编辑配置文件

编辑 `~/.claude/config.yaml`，添加以下配置：

```yaml
version: 0.8
auto_login:
  enabled: true           # 启用自动登录
  aiqicha:
    username: '13800138000'  # 爱企查账号（手机号）
    password: 'your_password'  # 爱企查密码
```

### 2. 完整配置示例

```yaml
version: 0.8
user_agent: ""
app:
  miit_api: ''
api:
  api: ':31000'
  mcp: 'http://localhost:8080'
cookies:
  aiqicha: ''              # 可以留空，程序会自动登录获取
  tianyancha: ''
  tycid: ''
  auth_token: ''
  tyc_api_token: ''
  risk_bird: ''
  qimai: ''
auto_login:
  enabled: true
  aiqicha:
    username: '13800138000'
    password: 'your_password'
```

## 使用场景

### 场景1：首次使用（无Cookie）

```bash
# 配置账号密码后，直接运行
./enscan -n 小米 -type aqc

# 程序会自动：
# 1. 检测到Cookie为空
# 2. 使用账号密码自动登录
# 3. 获取Cookie并保存到配置文件
# 4. 执行查询
```

### 场景2：Cookie失效

```bash
# 运行过程中Cookie失效
./enscan -n 小米 -type aqc

# 程序会自动：
# 1. 检测到401/登录提示
# 2. 自动重新登录
# 3. 更新Cookie
# 4. 重试请求
```

### 场景3：混合模式

你可以同时保留手动Cookie和自动登录配置：

```yaml
cookies:
  aiqicha: 'your_manual_cookie'  # 优先使用手动Cookie
auto_login:
  enabled: true                   # Cookie失效时自动登录
  aiqicha:
    username: '13800138000'
    password: 'your_password'
```

## Docker部署

### 轻量级镜像（无浏览器，仅支持手动Cookie）

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -o enscan .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /build/enscan /app/enscan
WORKDIR /app
CMD ["./enscan"]
```

### 完整镜像（支持自动登录）

```dockerfile
FROM golang:1.26 AS builder
WORKDIR /build
COPY . .
RUN go build -o enscan .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    chromium \
    chromium-driver \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ENV CHROME_BIN=/usr/bin/chromium

COPY --from=builder /build/enscan /app/enscan
WORKDIR /app
CMD ["./enscan"]
```

构建并运行：

```bash
# 构建镜像
docker build -t enscan:auto-login -f Dockerfile.auto-login .

# 运行（挂载配置文件）
docker run -v ~/.claude:/root/.claude enscan:auto-login -n 小米 -type aqc
```

## 注意事项

### 1. 验证码问题

当前版本的自动登录可能无法处理以下验证码：
- 滑块验证码
- 图形验证码
- 短信验证码

**解决方案**：
- 方案1：使用稳定IP，减少触发验证码的概率
- 方案2：保留手动Cookie配置作为备用
- 方案3：登录时手动完成验证码（未来版本可能支持）

### 2. 账号安全

- 配置文件包含明文密码，请确保文件权限安全：
  ```bash
  chmod 600 ~/.claude/config.yaml
  ```
- 建议使用独立的测试账号
- 不要在公共仓库中提交包含真实账号密码的配置文件

### 3. 登录频率

频繁的自动登录可能触发平台风控，建议：
- Cookie通常有效期较长，不会频繁失效
- 设置合理的请求延迟（`-delay` 参数）
- 避免短时间内大量请求

### 4. 资源消耗

自动登录需要启动headless Chrome：
- 内存占用：约100-200MB
- 登录耗时：约5-10秒
- 仅在Cookie为空/失效时才会触发

## 故障排查

### 问题1：自动登录失败

```
【AQC】自动登录失败: 登录流程执行失败
```

**可能原因**：
- 网络问题
- 账号密码错误
- 需要验证码
- Chromium未安装

**解决方法**：
```bash
# 检查Chromium是否安装
chromium --version  # 或 chromium-browser --version

# Alpine Linux
apk add chromium

# Debian/Ubuntu
apt-get install chromium

# 启用Debug日志查看详细信息
./enscan -n 小米 -type aqc --debug
```

### 问题2：Docker环境无法启动浏览器

```
【AQC】自动登录失败: chrome failed to start
```

**解决方法**：
- 确保使用完整镜像（包含Chromium）
- 添加必要的Docker运行参数：
  ```bash
  docker run --cap-add=SYS_ADMIN \
    -v ~/.claude:/root/.claude \
    enscan:auto-login -n 小米 -type aqc
  ```

### 问题3：Cookie保存失败

```
【AQC】保存Cookie失败: permission denied
```

**解决方法**：
```bash
# 检查配置文件权限
ls -la ~/.claude/config.yaml

# 修复权限
chmod 644 ~/.claude/config.yaml
```

## 版本兼容性

- 配置文件版本：0.8
- 旧版本配置（0.7）会自动升级，不影响现有功能
- 如果不需要自动登录，保持 `auto_login.enabled: false` 即可

## 未来计划

- [ ] 支持天眼查(TYC)自动登录
- [ ] 支持快查(KC)自动登录
- [ ] 支持风鸟(RB)自动登录
- [ ] 验证码识别（接入打码平台）
- [ ] Cookie有效期智能预测
- [ ] 登录状态持久化

## 技术实现

- 使用 `chromedp` 驱动headless Chrome
- 通过DOM选择器模拟用户登录操作
- 登录成功后提取Cookie并保存到配置文件
- 请求失败时自动检测失效原因并重试

## 贡献

如果你在使用中遇到问题或有改进建议，欢迎提交Issue或Pull Request。
