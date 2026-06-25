# 自动登录功能设计变更文档

## 变更概述

**版本**: 0.8  
**日期**: 2026-06-22  
**类型**: 功能增强  
**影响范围**: 风鸟(RiskBird)数据源  

本次变更为 ENScan_GO 添加了基于 chromedp 的自动登录功能，目前仅支持风鸟数据源。

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

### 为何仅支持风鸟？

基于GitHub调研和实际测试，不同数据源的登录难度差异巨大：

| 数据源 | 登录方式 | 验证码风险 | 实现难度 | 是否支持 |
|--------|----------|-----------|----------|----------|
| **风鸟** | 账号密码 | 低 | 简单 | ✅ 已实现 |
| **爱企查** | 百度通行证 | 高（滑块/图形） | 困难 | ❌ 不支持 |
| **天眼查** | 短信验证码 | 高 | 困难 | ❌ 不支持 |
| **快查** | 账号密码 | 中 | 中等 | ⏳ 待实现 |

**爱企查特殊限制：**
- 使用百度通行证（passport.baidu.com）
- 严格的反爬机制和设备指纹检测
- 频繁触发滑块验证码
- Cookie与IP强绑定
- 远程Linux无人值守环境无法处理验证码

**决策：** 放弃爱企查自动登录，仅实现风鸟等简单数据源。

---

## 架构设计

### 风鸟自动登录流程

```
用户配置账号密码
  ↓
程序启动 chromedp
  ↓
访问 riskbird.com
  ↓
点击登录触发弹窗
  ↓
切换到"密码登录"标签
  ↓
输入手机号和密码
  ↓
点击"登 录"按钮
  ↓
等待登录完成（5秒）
  ↓
提取 document.cookie
  ↓
保存到配置文件
```

### 核心组件

#### 1. RiskBird Login Manager (`internal/riskbird/auto_login.go`)

**职责**：
- 风鸟自动登录流程
- Cookie 提取和返回
- 登录状态验证

**关键方法**：
```go
type RBLoginManager struct {
    config *common.ENConfig
}

func NewRBLoginManager(config *common.ENConfig) *RBLoginManager
func (m *RBLoginManager) AutoLogin(username, password string) (string, error)
```

**DOM选择器（基于实际测试）**：
```go
loginDialog: `.el-overlay-dialog`              // 登录弹窗
passwordTab: `.tab-item:contains("密码登录")`   // 密码登录标签
phoneInput: `input[name="uaername"]`            // 手机号输入框
passwordInput: `input[name="password"]`         // 密码输入框
submitButton: `button.el-button--primary:contains("登 录")` // 登录按钮
```

#### 2. 配置结构扩展 (`common/config.go`)

**新增字段**：
```go
type ENConfig struct {
    // ... 原有字段
    AutoLogin struct {
        Enabled bool `yaml:"enabled"`
        RiskBird struct {
            Username string `yaml:"username"`
            Password string `yaml:"password"`
        } `yaml:"riskbird"`
    } `yaml:"auto_login"`
}
```

**配置文件版本**：`0.7` → `0.8`

---

## 实现细节

### 风鸟登录流程

```
1. 启动 headless Chrome
   ↓
2. 访问 riskbird.com
   ↓
3. 注入反检测脚本
   ↓
4. 点击登录按钮（触发弹窗）
   ↓
5. 等待弹窗出现 (.el-overlay-dialog)
   ↓
6. 切换到"密码登录"标签
   ↓
7. 输入手机号 (input[name="uaername"])
   ↓
8. 输入密码 (input[name="password"])
   ↓
9. 点击"登 录"按钮
   ↓
10. 等待登录完成（5秒）
   ↓
11. 验证登录状态（检查弹窗是否关闭）
   ↓
12. 提取 document.cookie
   ↓
13. 返回Cookie字符串
```

### 反检测措施

```javascript
// 注入到页面的脚本
Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en'] });
window.chrome = { runtime: {} };
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
  riskbird:
    username: ''
    password: ''
cookies:
  risk_bird: ''  # Cookie优先，自动登录作为备用
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

### ⚠️ 功能状态：实验性（不推荐生产使用）

**重要说明：** 该功能目前为**实验性质**，未经过充分的实际环境测试，存在以下严重限制：

### 当前版本限制

1. **仅支持风鸟**：爱企查、天眼查、快查暂不支持
2. **验证码风险**：遇到验证码会失败
3. **需要Chromium环境**：Docker需要完整镜像
4. **无代理支持**：自动登录不支持代理配置

### 不适用场景

- 需要短信验证码登录的数据源
- 频繁触发风控的环境
- 对安全性要求极高（明文密码）

---

## 未来规划

### 短期（v0.9）

- [ ] 支持快查自动登录
- [ ] Cookie持久化到配置文件
- [ ] 增加登录日志详细记录

### 中期（v1.0）

- [ ] 研究天眼查自动登录可行性
- [ ] Cookie 有效期智能预测
- [ ] 支持代理配置

### 长期

- [ ] 加密存储密码
- [ ] 多账号轮换机制
- [ ] 支持更多数据源

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
