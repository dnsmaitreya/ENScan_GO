package common

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/wgpsec/ENScan/common/gologger"
	"github.com/wgpsec/ENScan/common/utils"
	"gopkg.in/yaml.v3"
)

// CookieManager Cookie管理器
type CookieManager struct {
	config     *ENConfig
	configPath string
	mu         sync.RWMutex
}

// NewCookieManager 创建Cookie管理器
func NewCookieManager(config *ENConfig) *CookieManager {
	return &CookieManager{
		config:     config,
		configPath: utils.GetConfigPath() + "/config.yaml",
	}
}

// GetCookie 获取Cookie，如果失效则尝试自动登录
func (cm *CookieManager) GetCookie(source string) (string, error) {
	cm.mu.RLock()
	cookie := cm.getCookieFromConfig(source)
	cm.mu.RUnlock()

	// 如果Cookie存在，先尝试使用
	if cookie != "" {
		return cookie, nil
	}

	// Cookie为空且启用了自动登录
	if cm.config.AutoLogin.Enabled {
		gologger.Info().Msgf("【%s】Cookie为空，尝试自动登录...", strings.ToUpper(source))
		if err := cm.AutoLogin(source); err != nil {
			return "", fmt.Errorf("自动登录失败: %v", err)
		}

		// 重新读取Cookie
		cm.mu.RLock()
		cookie = cm.getCookieFromConfig(source)
		cm.mu.RUnlock()

		if cookie == "" {
			return "", fmt.Errorf("自动登录后Cookie仍为空")
		}
		return cookie, nil
	}

	return "", fmt.Errorf("【%s】Cookie为空且未启用自动登录，请手动配置", strings.ToUpper(source))
}

// getCookieFromConfig 从配置中获取Cookie
func (cm *CookieManager) getCookieFromConfig(source string) string {
	switch source {
	case "aqc":
		return cm.config.Cookies.Aiqicha
	case "tyc":
		return cm.config.Cookies.Tianyancha
	case "rb":
		return cm.config.Cookies.RiskBird
	case "kc":
		return cm.config.Cookies.KuaiCha
	default:
		return ""
	}
}

// AutoLogin 自动登录
func (cm *CookieManager) AutoLogin(source string) error {
	switch source {
	case "aqc":
		return cm.loginAiqicha()
	default:
		return fmt.Errorf("不支持【%s】的自动登录", source)
	}
}

// loginAiqicha 爱企查自动登录
func (cm *CookieManager) loginAiqicha() error {
	username := cm.config.AutoLogin.Aiqicha.Username
	password := cm.config.AutoLogin.Aiqicha.Password

	if username == "" || password == "" {
		return fmt.Errorf("爱企查账号或密码未配置")
	}

	gologger.Info().Msgf("【AQC】开始自动登录，账号: %s", username)

	// 配置chromedp选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(func(s string, i ...interface{}) {
		gologger.Debug().Msgf("[chromedp] "+s, i...)
	}))
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var cookies string

	// 执行登录流程
	err := chromedp.Run(ctx,
		// 访问登录页面
		chromedp.Navigate("https://aiqicha.baidu.com/"),
		chromedp.Sleep(2*time.Second),

		// 点击登录按钮（可能需要调整选择器）
		chromedp.Click(`a[href*="login"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),

		// 切换到密码登录
		chromedp.Click(`//div[contains(text(),'密码登录')]`, chromedp.BySearch),
		chromedp.Sleep(1*time.Second),

		// 输入手机号
		chromedp.SendKeys(`input[placeholder*="手机号"]`, username, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// 输入密码
		chromedp.SendKeys(`input[type="password"]`, password, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// 点击登录按钮
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),

		// 等待登录完成（检查是否跳转到首页或出现用户信息）
		chromedp.Sleep(5*time.Second),

		// 提取Cookie
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 获取所有cookie
			err := chromedp.Evaluate(`document.cookie`, &cookies).Do(ctx)
			if err != nil {
				return fmt.Errorf("获取Cookie失败: %v", err)
			}

			gologger.Debug().Msgf("【AQC】获取到Cookie: %s", cookies)
			return nil
		}),
	)

	if err != nil {
		return fmt.Errorf("登录流程执行失败: %v", err)
	}

	if cookies == "" {
		return fmt.Errorf("登录后未能获取到Cookie，可能存在验证码或登录失败")
	}

	// 保存Cookie到配置文件
	if err := cm.saveCookie("aqc", cookies); err != nil {
		return fmt.Errorf("保存Cookie失败: %v", err)
	}

	gologger.Info().Msgf("【AQC】自动登录成功，Cookie已更新")
	return nil
}

// saveCookie 保存Cookie到配置文件
func (cm *CookieManager) saveCookie(source, cookie string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 读取配置文件
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析YAML
	var config ENConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 更新Cookie
	switch source {
	case "aqc":
		config.Cookies.Aiqicha = cookie
		cm.config.Cookies.Aiqicha = cookie
	case "tyc":
		config.Cookies.Tianyancha = cookie
		cm.config.Cookies.Tianyancha = cookie
	case "rb":
		config.Cookies.RiskBird = cookie
		cm.config.Cookies.RiskBird = cookie
	case "kc":
		config.Cookies.KuaiCha = cookie
		cm.config.Cookies.KuaiCha = cookie
	}

	// 写回配置文件
	newData, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	if err := os.WriteFile(cm.configPath, newData, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	gologger.Debug().Msgf("【%s】Cookie已保存到配置文件", strings.ToUpper(source))
	return nil
}

// ValidateCookie 验证Cookie是否有效
func (cm *CookieManager) ValidateCookie(source, cookie string) bool {
	// TODO: 实现Cookie有效性检测逻辑
	// 可以发送一个简单的API请求来验证Cookie是否有效
	return cookie != ""
}
