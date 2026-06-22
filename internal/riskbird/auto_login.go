package riskbird

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/wgpsec/ENScan/common"
	"github.com/wgpsec/ENScan/common/gologger"
)

// RBLoginManager 风鸟登录管理器
type RBLoginManager struct {
	config *common.ENConfig
}

// NewRBLoginManager 创建风鸟登录管理器
func NewRBLoginManager(config *common.ENConfig) *RBLoginManager {
	return &RBLoginManager{
		config: config,
	}
}

// AutoLogin 自动登录风鸟
func (m *RBLoginManager) AutoLogin(username, password string) (string, error) {
	if username == "" || password == "" {
		return "", fmt.Errorf("风鸟账号或密码未配置")
	}

	gologger.Info().Msgf("【RB】开始自动登录，账号: %s", username)

	// 配置chromedp选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
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
		// 访问风鸟首页
		chromedp.Navigate("https://www.riskbird.com/"),
		chromedp.Sleep(3*time.Second),

		// 注入反检测脚本
		chromedp.Evaluate(`
			Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
			Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
			Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en'] });
			window.chrome = { runtime: {} };
		`, nil),

		// 点击登录按钮（触发弹窗）
		chromedp.Click(`a[href*="login"], button:contains("登录")`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),

		// 等待登录弹窗出现
		chromedp.WaitVisible(`.el-overlay-dialog`, chromedp.ByQuery),

		// 切换到"密码登录"标签（如果不是默认的话）
		chromedp.Click(`.tab-item:contains("密码登录")`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// 输入手机号（name="uaername"，注意是拼写错误）
		chromedp.SendKeys(`input[name="uaername"]`, username, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// 输入密码
		chromedp.SendKeys(`input[name="password"]`, password, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// 点击"登 录"按钮
		chromedp.Click(`button.el-button--primary:contains("登 录")`, chromedp.ByQuery),

		// 等待登录完成（检查弹窗是否关闭或跳转）
		chromedp.Sleep(5*time.Second),

		// 提取Cookie
		chromedp.ActionFunc(func(ctx context.Context) error {
			err := chromedp.Evaluate(`document.cookie`, &cookies).Do(ctx)
			if err != nil {
				return fmt.Errorf("获取Cookie失败: %v", err)
			}

			gologger.Debug().Msgf("【RB】获取到Cookie: %s", cookies)
			return nil
		}),

		// 验证登录是否成功（检查是否有登录后的元素）
		chromedp.ActionFunc(func(ctx context.Context) error {
			var loginSuccess bool
			err := chromedp.Evaluate(`
				// 检查是否有登录错误提示
				const errorMsg = document.querySelector('.el-message--error, .login-error');
				if (errorMsg) {
					return false;
				}

				// 检查弹窗是否还在（登录失败通常弹窗还在）
				const dialog = document.querySelector('.el-overlay-dialog');
				if (dialog && dialog.offsetWidth > 0) {
					return false;
				}

				// 登录成功，弹窗应该消失
				return true;
			`, &loginSuccess).Do(ctx)

			if err != nil {
				return fmt.Errorf("验证登录状态失败: %v", err)
			}

			if !loginSuccess {
				return fmt.Errorf("登录失败，请检查账号密码或可能存在验证码")
			}

			return nil
		}),
	)

	if err != nil {
		return "", fmt.Errorf("登录流程执行失败: %v", err)
	}

	if cookies == "" {
		return "", fmt.Errorf("登录后未能获取到Cookie")
	}

	gologger.Info().Msgf("【RB】自动登录成功")
	return cookies, nil
}
