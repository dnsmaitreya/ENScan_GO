package riskbird

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
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
//
// 风鸟登录弹窗 2025 改版（xs-login-* Vue 组件）流程：
//  1. 首页加载后登录弹窗自动弹出，默认显示「扫码登录」(二维码) 视图；
//  2. 点击左侧橙色图片切换按钮 (img.Login-mode-img / pass-login-*) 切换到表单视图；
//  3. 表单默认停在「验证码登录/注册」(短信验证码) 标签，需点击「密码登录」文字标签 (div.tab-item)；
//  4. 填写手机号 input[name="uaername"] 与密码 input[name="password"]；
//  5. 点击「登 录」按钮 button.login-form-item-btn（初始 disabled，填完后由 Vue 解禁）。
//
// 密码登录标签下无图形/滑块验证码。
func (m *RBLoginManager) AutoLogin(username, password string) (string, error) {
	if username == "" || password == "" {
		return "", fmt.Errorf("风鸟账号或密码未配置")
	}

	gologger.Info().Msgf("【RB】开始自动登录")

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

	err := chromedp.Run(ctx,
		// 注入反检测脚本——必须在导航前通过 CDP 注册，使其在每个新文档脚本执行前生效，
		// 否则页面自带的 bot 检测脚本会先于注入运行而失效。
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(`
				Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
				Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
				Object.defineProperty(navigator, 'languages', { get: () => ['zh-CN', 'zh', 'en'] });
				window.chrome = { runtime: {} };
			`).Do(ctx)
			return err
		}),

		// 访问风鸟首页
		chromedp.Navigate("https://www.riskbird.com/"),
		chromedp.Sleep(5*time.Second),

		// 等待登录弹窗出现
		chromedp.WaitVisible(`.xs-login-box`, chromedp.ByQuery),

		// 切换到表单视图：点击橙色图片切换按钮（仅当当前为二维码视图时点击，避免反向切回）
		chromedp.ActionFunc(func(ctx context.Context) error {
			var ok string
			if err := chromedp.Evaluate(`
				(function() {
					var img = document.querySelector('img.Login-mode-img');
					if (!img) {
						var imgs = document.querySelectorAll('img');
						for (var i = 0; i < imgs.length; i++) {
							if ((imgs[i].src || '').indexOf('pass-login') >= 0) { img = imgs[i]; break; }
						}
					}
					// status: clicked=已点击切换 / already-form=已是表单视图 / not-found=找不到切换控件
					if (!img) return 'not-found';
					if ((img.src || '').indexOf('pass-login') >= 0) { img.click(); return 'clicked'; }
					return 'already-form';
				})()
			`, &ok).Do(ctx); err != nil {
				return err
			}
			gologger.Debug().Msgf("【RB】切换到密码表单视图: %s", ok)
			if ok == "not-found" {
				return fmt.Errorf("未找到登录方式切换控件(img.Login-mode-img)，风鸟登录页结构可能再次变化")
			}
			return nil
		}),
		chromedp.Sleep(1500*time.Millisecond),

		// 切换到"密码登录"标签（文字标签 div.tab-item）
		chromedp.ActionFunc(func(ctx context.Context) error {
			var ok bool
			if err := chromedp.Evaluate(`
				(function() {
					var tabs = document.querySelectorAll('.list-tabs-box .tab-item, .tab-item');
					for (var i = 0; i < tabs.length; i++) {
						if (tabs[i].textContent.replace(/\s+/g, '') === '密码登录') {
							tabs[i].click(); return true;
						}
					}
					return false;
				})()
			`, &ok).Do(ctx); err != nil {
				return err
			}
			gologger.Debug().Msgf("【RB】切换到密码登录标签: %v", ok)
			if !ok {
				return fmt.Errorf("未找到「密码登录」标签，可能仍停留在二维码视图或页面结构变化")
			}
			return nil
		}),
		chromedp.Sleep(1200*time.Millisecond),

		// 等待密码输入框出现
		chromedp.WaitVisible(`.login-form input[name="password"]`, chromedp.ByQuery),

		// 输入手机号（name="uaername"，站点原始拼写如此）
		chromedp.SendKeys(`.login-form input[name="uaername"]`, username, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// 输入密码
		chromedp.SendKeys(`.login-form input[name="password"]`, password, chromedp.ByQuery),
		chromedp.Sleep(800*time.Millisecond),

		// 点击"登 录"按钮（填完后由 Vue 解除 disabled，轮询等待解禁后点击）
		chromedp.ActionFunc(func(ctx context.Context) error {
			for i := 0; i < 20; i++ {
				var clicked bool
				if err := chromedp.Evaluate(`
					(function() {
						var btn = document.querySelector('button.login-form-item-btn');
						if (!btn) {
							var btns = document.querySelectorAll('button.el-button--primary');
							for (var i = 0; i < btns.length; i++) {
								if (btns[i].textContent.replace(/\s+/g, '').indexOf('登录') >= 0) { btn = btns[i]; break; }
							}
						}
						if (btn && !btn.disabled) { btn.click(); return true; }
						return false;
					})()
				`, &clicked).Do(ctx); err != nil {
					return err
				}
				if clicked {
					gologger.Debug().Msgf("【RB】已点击登录按钮")
					return nil
				}
				// 等待时尊重 context 取消（超时立即返回，不再空等 300ms）
				select {
				case <-time.After(300 * time.Millisecond):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return fmt.Errorf("登录按钮始终为禁用状态（手机号/密码可能未正确填入）")
		}),

		// 等待登录完成（检查弹窗是否关闭或跳转）
		chromedp.Sleep(5*time.Second),

		// 通过CDP提取Cookie（包含HttpOnly）
		chromedp.ActionFunc(func(ctx context.Context) error {
			cdpCookies, err := network.GetCookies().
				WithURLs([]string{"https://www.riskbird.com"}).
				Do(ctx)
			if err != nil {
				return fmt.Errorf("获取Cookie失败: %v", err)
			}
			var parts []string
			for _, c := range cdpCookies {
				parts = append(parts, c.Name+"="+c.Value)
			}
			cookies = strings.Join(parts, "; ")
			gologger.Debug().Msgf("【RB】获取到 %d 个 Cookie", len(cdpCookies))
			return nil
		}),

		// 验证登录是否成功（弹窗关闭 / 无错误提示）
		chromedp.ActionFunc(func(ctx context.Context) error {
			var loginSuccess bool
			err := chromedp.Evaluate(`
				(function() {
					// 检查是否有登录错误提示
					const errorMsg = document.querySelector('.el-message--error, .login-error');
					if (errorMsg && errorMsg.offsetWidth > 0) {
						return false;
					}
					// 登录弹窗消失视为成功
					const box = document.querySelector('.xs-login-box');
					if (box && box.offsetWidth > 0) {
						return false;
					}
					return true;
				})()
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
