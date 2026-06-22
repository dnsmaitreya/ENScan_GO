# 自动登录功能设计变更文档

## 变更概述

**版本**: 0.8  
**日期**: 2026-06-22  
**类型**: 功能增强  
**影响范围**: 爱企查(AQC)数据源  

本次变更为 ENScan_GO 添加了基于 chromedp 的自动登录功能，解决了用户需要手动获取和更新 Cookie 的痛点。

---

## 需求背景

### 当前问题

1. **手动获取 Cookie 流程复杂**：用户需要登录浏览器 → 打开开发者工具 → 复制 Cookie → 粘贴到配置文件
2. **Cookie 频繁失效**：Cookie 过期后需要重复上述流程
3. **Docker 环境不友好**：无头环境下无法手动获取 Cookie
4. **批量任务中断**：长时间运行任务中 Cookie 失效导致任务中断

### 用户需求

用户希望能够配置一次账号密码后，程序自动完成登录和 Cookie 管理，无需人工干预。

---

## 设计方案

### 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                       ENScan_GO                              │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐         ┌──────────────────────────────┐  │
│  │  AQC Module  │────────▶│   Cookie Manager (NEW)       │  │
│  │              │         │                              │  │
│  │  req()       │         │  - GetCookie()               │  │
│  │  - 检测401   │◀────────│  - AutoLogin()               │  │
│  │  - 触发重登录 │         │  - ValidateCookie()          │  │
│  └──────────────┘         │  - SaveCookie()              │  │
│                           └───────────┬──────────────────┘  │
│                                       │                      │
│  ┌──────────────┐                     │                      │
│  │  ENConfig    │                     │                      │
│  │  (Updated)   │                     ▼                      │
│  │              │         ┌──────────────────────────────┐  │
│  │ + AutoLogin  │         │      chromedp                 │  │
│  │   - enabled  │         │   (Headless Chrome)           │  │
│  │   - username │         │                              │  │
│  │   - password │         │  - Navigate to login page    │  │
│  └──────────────┘         │  - Fill username/password    │  │
│                           │  - Extract cookies           │  │
│                           └──────────────────────────────┘  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### 核心组件

#### 1. Cookie Manager (`common/cookie_manager.go`)

**职责**：
- Cookie 生命周期管理
- 自动登录流程编排
- Cookie 持久化（保存到配置文件）

**关键方法**：
```go
type CookieManager struct {
    config     *ENConfig
    configPath string
    mu         sync.RWMutex
}

func (cm *CookieManager) GetCookie(source string) (string, error)
func (cm *CookieManager) AutoLogin(source string) error
func (cm *CookieManager) loginAiqicha() error
func (cm *CookieManager) saveCookie(source, cookie string) error
```

#### 2. 配置结构扩展 (`common/config.go`)

**新增字段**：
```go
type ENConfig struct {
    // ... 原有字段
    AutoLogin struct {
        Enabled bool `yaml:"enabled"`
        Aiqicha struct {
            Username string `yaml:"username"`
            Password string `yaml:"password"`
        } `yaml:"aiqicha"`
    } `yaml:"auto_login"`
}
```

**配置文件版本**：`0.7` → `0.8`

#### 3. AQC 模块改造 (`internal/aiqicha/bean.go`)

**变更点**：
- `req()` 方法增加 Cookie 失效检测
- 检测到 401/登录提示时触发自动重登录
- 登录成功后自动重试请求

---

## 实现细节

### 登录流程

```
┌─────────────────────────────────────────────────────────────┐
│                    自动登录流程                              │
└─────────────────────────────────────────────────────────────┘

1. 用户请求数据
   │
   ├─▶ 检查 Cookie 是否存在
   │   │
   │   ├─ 存在 ─▶ 使用现有 Cookie
   │   │
   │   └─ 不存在 ─▶ 检查是否启用自动登录
   │                │
   │                ├─ 未启用 ─▶ 报错提示用户手动配置
   │                │
   │                └─ 已启用 ─▶ 执行自动登录
   │
2. 自动登录执行
   │
   ├─▶ 启动 headless Chrome
   │
   ├─▶ 访问 aiqicha.baidu.com
   │
   ├─▶ 点击"登录"按钮
   │
   ├─▶ 切换到"密码登录"
   │
   ├─▶ 输入手机号和密码
   │
   ├─▶ 提交表单
   │
   ├─▶ 等待登录完成（5秒）
   │
   ├─▶ 提取 document.cookie
   │
   └─▶ 保存到 config.yaml
   
3. Cookie 失效检测
   │
   ├─▶ 请求返回 401
   │   └─▶ 触发重登录
   │
   ├─▶ 响应包含"请登录"/"未登录"
   │   └─▶ 触发重登录
   │
   └─▶ 重登录成功后重试原请求
```

### Cookie 有效性检测

**触发条件**：
1. HTTP 状态码 401
2. HTTP 状态码 302（重定向到登录页）
3. 响应内容包含关键词：`请登录`、`未登录`

**处理逻辑**：
```go
if resp.StatusCode == 401 || 
   strings.Contains(resp.String(), "请登录") {
    if h.Options.ENConfig.AutoLogin.Enabled {
        // 自动重新登录
        cookieManager.AutoLogin("aqc")
        // 重试请求
        return h.req(url)
    } else {
        // 提示用户手动更新
        gologger.Error().Msgf("Cookie失效，请重新获取")
    }
}
```

---

## 技术选型

### chromedp vs selenium

| 方案 | 优点 | 缺点 | 选择 |
|------|------|------|------|
| **chromedp** | • Go 原生库<br>• 无需外部依赖<br>• 性能好 | • 调试相对困难 | ✅ 采用 |
| selenium | • 生态成熟<br>• 跨语言 | • 需要 WebDriver<br>• 部署复杂 | ❌ |

### 依赖添加

```bash
go get github.com/chromedp/chromedp@latest
```

**镜像体积影响**：
- 轻量镜像（无浏览器）：~20MB
- 完整镜像（含 Chromium）：~200MB

---

## 向后兼容性

### 配置文件兼容

**版本 0.7（旧）**：
```yaml
version: 0.7
cookies:
  aiqicha: 'manual_cookie'
```

**版本 0.8（新）**：
```yaml
version: 0.8
auto_login:
  enabled: false  # 默认关闭，不影响现有用户
  aiqicha:
    username: ''
    password: ''
cookies:
  aiqicha: 'manual_cookie'  # 仍然支持手动配置
```

### 功能降级

如果 Chromium 未安装或自动登录失败：
1. 回退到手动 Cookie 模式
2. 提示用户安装依赖或手动配置
3. 不影响程序其他功能

---

## 安全性考虑

### 风险点

1. **明文密码存储**：配置文件包含明文密码
2. **登录频率限制**：频繁登录可能触发平台风控
3. **验证码问题**：当前版本无法处理验证码

### 安全措施

1. **文件权限**：
   ```bash
   chmod 600 ~/.claude/config.yaml  # 仅所有者可读写
   ```

2. **最小化登录次数**：
   - 优先使用现有 Cookie
   - Cookie 失效才触发重登录
   - 登录成功后持久化保存

3. **敏感信息提示**：
   - 文档明确提示密码安全风险
   - 建议使用独立测试账号
   - 警告不要提交包含密码的配置文件

4. **未来改进**：
   - [ ] 支持加密存储密码
   - [ ] 集成验证码识别服务
   - [ ] Cookie 有效期智能预测

---

## 测试策略

### 测试场景

| 场景 | 输入 | 期望输出 | 测试方法 |
|------|------|----------|----------|
| **首次使用（无Cookie）** | `auto_login.enabled: true`<br>配置账号密码 | 自动登录并获取 Cookie | 清空 Cookie 运行 |
| **Cookie 失效** | 配置无效 Cookie | 自动重新登录 | 手动设置过期 Cookie |
| **自动登录失败** | 错误的账号密码 | 提示失败，使用手动模式 | 配置错误密码 |
| **验证码拦截** | 触发验证码 | 提示需要人工干预 | （需要真实环境） |
| **向后兼容** | 0.7 版本配置文件 | 正常运行，不触发自动登录 | 使用旧配置文件 |

### 测试脚本

提供自动化测试脚本：`scripts/test_auto_login.sh`

```bash
# 检查配置文件
# 检查 Chromium 依赖
# 验证配置正确性
./scripts/test_auto_login.sh
```

---

## Docker 支持

### 两种镜像

**1. 轻量镜像** (`Dockerfile.lite`)
- 基于 Alpine Linux
- 仅支持手动 Cookie
- 镜像大小：~20MB

**2. 完整镜像** (`Dockerfile.auto-login`)
- 基于 Debian + Chromium
- 支持自动登录
- 镜像大小：~200MB

### 部署示例

```bash
# 构建完整镜像
docker build -t enscan:auto-login -f Dockerfile.auto-login .

# 运行（挂载配置文件）
docker run -v ~/.claude:/root/.claude \
  enscan:auto-login -n 小米 -type aqc
```

---

## 已知限制

### 当前版本限制

1. **仅支持爱企查**：其他数据源（天眼查、快查、风鸟）暂不支持
2. **无验证码处理**：遇到验证码会失败
3. **登录选择器硬编码**：网站改版可能导致失效
4. **无代理支持**：自动登录不支持代理配置

### 不适用场景

- 账号需要短信验证码登录
- IP 地址频繁触发风控
- 网络环境不稳定
- 对安全性要求极高（明文密码）

---

## 未来规划

### 短期（v0.9）

- [ ] 支持天眼查自动登录
- [ ] 优化登录选择器（更鲁棒）
- [ ] 增加登录日志详细记录

### 中期（v1.0）

- [ ] 支持快查、风鸟自动登录
- [ ] 集成打码平台（处理验证码）
- [ ] Cookie 有效期智能预测
- [ ] 支持代理配置

### 长期

- [ ] 加密存储密码
- [ ] 多账号轮换机制
- [ ] 登录状态监控仪表板
- [ ] 支持二维码登录

---

## 变更影响评估

### 对现有用户

- ✅ **无影响**：默认禁用，不影响现有手动 Cookie 流程
- ✅ **可选启用**：用户可自行选择是否开启自动登录
- ✅ **配置兼容**：0.7 配置文件自动兼容

### 对新用户

- ✅ **更简单**：配置一次账号密码即可
- ✅ **更稳定**：Cookie 失效自动恢复
- ⚠️ **需要依赖**：Docker 环境需要安装 Chromium

### 对开发者

- ✅ **模块化设计**：Cookie 管理独立模块，易于扩展到其他数据源
- ✅ **接口统一**：其他数据源可复用 `CookieManager`
- ⚠️ **维护成本**：需要跟进网站登录页面变化

---

## 文档清单

- ✅ `docs/AUTO_LOGIN.md` - 用户使用指南
- ✅ `docs/DESIGN_CHANGES.md` - 本设计文档
- ✅ `scripts/test_auto_login.sh` - 测试脚本
- ✅ `README.md` - 更新使用说明

---

## 技术债务

1. **登录选择器维护**：需要定期检查网站是否改版
2. **错误处理完善**：登录失败的各种边界情况
3. **单元测试覆盖**：Cookie Manager 需要添加单元测试
4. **日志规范化**：自动登录流程日志需要统一格式

---

## 审批记录

| 角色 | 姓名 | 审批意见 | 日期 |
|------|------|----------|------|
| 开发者 | Claude Code | 设计完成 | 2026-06-22 |
| 用户 | @dnsmaitreya | 需求确认 | 2026-06-22 |

---

## 参考资料

- [chromedp 官方文档](https://github.com/chromedp/chromedp)
- [ENScan_GO 原项目](https://github.com/wgpsec/ENScan_GO)
- [爱企查官网](https://aiqicha.baidu.com/)
