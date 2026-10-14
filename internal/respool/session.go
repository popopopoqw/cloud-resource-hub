package respool

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// sessionManager 论坛登录态管理：chromedp 驱动本机 Chrome 完成登录
// （极验在真实浏览器指纹下多为无感通过），cookie 持久化到本地文件，
// 跨重启复用；失效时自动重登。
type sessionManager struct {
	base     string
	account  string
	password string
	savePath string
	headed   bool
}

type savedSession struct {
	Cookie    string    `json:"cookie"`
	UpdatedAt time.Time `json:"updated_at"`
}

// newSessionManager 创建。account/password 任一为空则无法自动登录。
func newSessionManager(base, account, password, dataDir string, headed bool) *sessionManager {
	return &sessionManager{
		base: strings.TrimRight(base, "/"), account: account, password: password,
		savePath: filepath.Join(dataDir, "respool_session.json"),
	}
}

// load 读取持久化 cookie；不存在返回空。
func (m *sessionManager) load() (string, time.Time, error) {
	b, err := os.ReadFile(m.savePath)
	if err != nil {
		return "", time.Time{}, err
	}
	var s savedSession
	if err := json.Unmarshal(b, &s); err != nil {
		return "", time.Time{}, err
	}
	return s.Cookie, s.UpdatedAt, nil
}

// save 持久化 cookie。
func (m *sessionManager) save(cookie string) error {
	if err := os.MkdirAll(filepath.Dir(m.savePath), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(savedSession{Cookie: cookie, UpdatedAt: time.Now()}, "", "  ")
	return os.WriteFile(m.savePath, b, 0o600)
}

// login 用 Chrome 完成登录并返回 cookie 串（bbs_token 等）。
// 全自动分层策略（按优先级）：
//  1. 无感通过：极验低风险场景直接 success，零操作
//  2. 自动滑块：截图缺口检测 + 人类轨迹模拟（多轮重试）
//  3. 打码平台：CaptchaAPI 配置时调用第三方识别缺口（预留）
//  4. 人工兜底：headed=true 弹窗让用户拖（极少触发）
//
// 浏览器 profile 持久化（user-data-dir），积累极验信任度。
func (m *sessionManager) login(ctx context.Context, headed bool) (string, error) {
	if m.account == "" || m.password == "" {
		return "", fmt.Errorf("未配置论坛账号（RESPOOL_ACCOUNT/RESPOOL_PASSWORD）")
	}
	m.headed = headed // 同步到实例，handleCaptcha 层4 依赖此值

	// 持久化浏览器 profile：保持指纹连续性，降低极验风险评分
	profileDir := filepath.Join(filepath.Dir(m.savePath), "chrome-profile")
	_ = os.MkdirAll(profileDir, 0o755)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", !headed),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent(ua),
		chromedp.UserDataDir(profileDir), // 持久化：cookie/指纹/信任度累积
	)
	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()
	browserCtx, cancel2 := chromedp.NewContext(allocCtx)
	defer cancel2()
	browserCtx, cancel3 := context.WithTimeout(browserCtx, 300*time.Second)
	defer cancel3()

	loginURL := m.base + "/user-login.htm"
	fmt.Println("[respool] 自动登录中（浏览器 profile 持久化，信任度累积）…")

	var cookies []*network.CookieParam
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(loginURL),
		chromedp.WaitVisible(`input[name="email"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="email"]`, m.account, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="password"]`, m.password, chromedp.ByQuery),
		chromedp.Sleep(1500*time.Millisecond), // 给极验加载时间
		// 点击极验按钮触发验证
		chromedp.ActionFunc(func(c context.Context) error {
			_ = chromedp.Click(`[class*="geetest_radar"], [class*="geetest_btn"]`, chromedp.ByQuery).Do(c)
			return nil
		}),
		chromedp.Sleep(2500*time.Millisecond), // 等待极验弹出/无感判定
		// 分层验证处理
		chromedp.ActionFunc(func(c context.Context) error {
			return m.handleCaptcha(c)
		}),
		// 验证通过后提交登录
		chromedp.ActionFunc(func(c context.Context) error {
			fmt.Println("[respool] 提交登录表单…")
			return chromedp.Click(`button[type="submit"], #submit`, chromedp.ByQuery).Do(c)
		}),
		chromedp.Sleep(3000*time.Millisecond),
		chromedp.ActionFunc(func(c context.Context) error {
			cs, err := network.GetCookies().Do(c)
			if err != nil {
				return err
			}
			for _, ck := range cs {
				cookies = append(cookies, &network.CookieParam{
					Name: ck.Name, Value: ck.Value, Domain: ck.Domain, Path: ck.Path,
				})
			}
			return nil
		}),
	)
	if err != nil {
		return "", fmt.Errorf("自动登录失败: %w", err)
	}
	var parts []string
	for _, ck := range cookies {
		if ck.Name == "" {
			continue
		}
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	cookie := strings.Join(parts, "; ")
	if !strings.Contains(cookie, "bbs_token") {
		return "", fmt.Errorf("登录后未获得会话（bbs_token）：账号或密码可能有误")
	}
	fmt.Println("[respool] 登录成功，会话已获取")
	return cookie, nil
}

// handleCaptcha 分层处理极验验证。
func (m *sessionManager) handleCaptcha(c context.Context) error {
	// 层1：检查是否已无感通过
	var passed bool
	_ = chromedp.Evaluate(`(() => { const r = document.querySelector('[class*="geetest_radar"]'); return !!(r && r.className.includes('success')); })()`, &passed).Do(c)
	if passed {
		fmt.Println("[respool] 极验无感通过（零操作）")
		return nil
	}

	// 层2：自动滑块（多轮重试）
	for round := 1; round <= 3; round++ {
		fmt.Printf("[respool] 滑块自动破解第 %d/3 轮…\n", round)
		if m.autoSlide(c) {
			chromedp.Sleep(2 * time.Second).Do(c)
			_ = chromedp.Evaluate(`(() => { const r = document.querySelector('[class*="geetest_radar"]'); return !!(r && r.className.includes('success')); })()`, &passed).Do(c)
			if passed {
				fmt.Printf("[respool] 第 %d 轮滑块通过\n", round)
				return nil
			}
		}
		// 失败刷新重试
		_ = chromedp.Click(`.geetest_refresh`, chromedp.ByQuery).Do(c)
		chromedp.Sleep(2 * time.Second).Do(c)
	}

	// 层3：打码平台（预留接口，CaptchaAPI 配置时调用）
	// TODO: 接入第三方打码平台（如超级鹰/云码），传入截图返回缺口坐标

	// 层4：headed 模式下等待人工拖（15秒轮询，极验通过即继续）；headless 直接报错
	if m.headed {
		fmt.Println("[respool] 自动滑块未通过，请在弹出的窗口中手动拖动滑块（15秒内）…")
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			var ok bool
			_ = chromedp.Evaluate(`(() => { const r = document.querySelector('[class*="geetest_radar"]'); return !!(r && r.className.includes('success')); })()`, &ok).Do(c)
			if ok {
				fmt.Println("[respool] 人工滑块已通过")
				return nil
			}
			select {
			case <-c.Done():
				return c.Err()
			case <-time.After(time.Second):
			}
		}
		return fmt.Errorf("等待人工滑块超时（15秒）")
	}
	return fmt.Errorf("自动滑块3轮未通过：设 RESPOOL_HEADED=true 弹窗人工兜底（仅首次需要，之后 cookie 持久化）")
}

// autoSlide 自动拖动极验滑块：截图缺口检测 + 人类行为轨迹。
func (m *sessionManager) autoSlide(c context.Context) bool {
	// 获取滑块几何
	var infoJSON string
	js := `(() => {
		const btn = document.querySelector('[class*="geetest_btn"]');
		const bg = document.querySelector('[class*="geetest_bg"]');
		const sl = document.querySelector('[class*="geetest_slice"]');
		if (!btn || !bg || !sl) return JSON.stringify({err: 'no-elem'});
		const b = btn.getBoundingClientRect(), r = bg.getBoundingClientRect();
		const bi = getComputedStyle(bg).backgroundImage;
		const m = bi && (bi.match(/url\("([^"]+)"\)/) || bi.match(/url\(([^)"]+)\)/));
		return JSON.stringify({
			bg: m ? m[1] : '', bgw: r.width, bgx: r.x,
			slicel: parseFloat(sl.style.left) || 0, slicew: parseFloat(sl.style.width) || 0,
			bx: b.x + b.width/2, by: b.y + b.height/2,
		});
	})()`
	if err := chromedp.Evaluate(js, &infoJSON).Do(c); err != nil {
		fmt.Printf("[respool] autoSlide evaluate err: %v\n", err)
		return false
	}
	fmt.Printf("[respool] autoSlide infoJSON: %s\n", infoJSON[:min(120, len(infoJSON))])
	var info struct {
		Err    string  `json:"err"`
		Bg     string  `json:"bg"`
		Bgw    float64 `json:"bgw"`
		Slicel float64 `json:"slicel"`
		Slicew float64 `json:"slicew"`
		BX     float64 `json:"bx"`
		BY     float64 `json:"by"`
	}
	if err := json.Unmarshal([]byte(infoJSON), &info); err != nil {
		fmt.Printf("[respool] autoSlide json err: %v\n", err)
		return false
	}
	if info.Err != "" || info.Bg == "" {
		fmt.Printf("[respool] autoSlide info err=%q bg=%q\n", info.Err, info.Bg[:min(30, len(info.Bg))])
		return false
	}

	// 下载背景图 → 缺口检测
	img, w, err := fetchImage(info.Bg)
	if err != nil {
		fmt.Printf("[respool] autoSlide fetchImage err: %v\n", err)
		return false
	}
	gapX := detectGap(img)
	if gapX <= 0 {
		return false
	}
	scale := info.Bgw / float64(w)
	dist := (float64(gapX) - info.Slicew/2) * scale
	fmt.Printf("[respool] 缺口=%dpx 拖动=%.0fpx\n", gapX, dist)

	// 人类行为轨迹拖动（多层随机化）
	_ = input.DispatchMouseEvent(input.MousePressed, info.BX, info.BY).
		WithButton(input.Left).WithClickCount(1).Do(c)

	steps := 40 + rand.Intn(15)
	var yJit float64
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		// easeInOutQuad + 随机扰动
		var ease float64
		if t < 0.5 {
			ease = 2 * t * t
		} else {
			ease = 1 - math.Pow(-2*t+2, 2)/2
		}
		x := info.BX + dist*ease
		yJit += (rand.Float64() - 0.5) * 2.0
		if math.Abs(yJit) > 3.5 {
			yJit = -yJit / 2
		}
		y := info.BY + yJit
		_ = input.DispatchMouseEvent(input.MouseMoved, x, y).Do(c)
		time.Sleep(time.Duration(8+rand.Intn(25)) * time.Millisecond)
	}
	time.Sleep(time.Duration(80+rand.Intn(150)) * time.Millisecond)
	_ = input.DispatchMouseEvent(input.MouseMoved, info.BX+dist+float64(rand.Intn(4)-2), info.BY).Do(c)
	time.Sleep(time.Duration(50+rand.Intn(80)) * time.Millisecond)
	_ = input.DispatchMouseEvent(input.MouseReleased, info.BX+dist, info.BY).
		WithButton(input.Left).WithClickCount(1).Do(c)
	chromedp.Sleep(1500 * time.Millisecond).Do(c)
	return true
}

// detectGap 扫描极验背景图找缺口（中带亮度暗平台法）。
func detectGap(img image.Image) int {
	b := img.Bounds()
	y0, y1 := b.Min.Y+b.Dy()/4, b.Min.Y+b.Dy()*3/4
	avgs := make([]float64, b.Dx())
	for x := 0; x < b.Dx(); x++ {
		sum, n := 0.0, 0
		for y := y0; y < y1; y++ {
			r, g, bl, _ := img.At(b.Min.X+x, y).RGBA()
			sum += (float64(r) + float64(g) + float64(bl)) / 3 / 257
			n++
		}
		avgs[x] = sum / float64(n)
	}
	start := int(float64(len(avgs)) * 0.4)
	bestAvg, bestX := math.MaxFloat64, -1
	runSum, runLen, runStart := 0.0, 0, -1
	flush := func() {
		if runLen >= 16 {
			avg := runSum / float64(runLen)
			if avg < bestAvg {
				bestAvg, bestX = avg, runStart+runLen/2
			}
		}
	}
	for x := 1; x < len(avgs); x++ {
		if runLen > 0 && math.Abs(avgs[x]-avgs[x-1]) > 13 {
			flush()
			runSum, runLen, runStart = 0, 0, -1
		}
		if runLen == 0 {
			runStart = x
		}
		runSum += avgs[x]
		runLen++
	}
	flush()
	if bestX < start {
		return -1
	}
	return bestX
}

// fetchImage 下载极验背景图。
func fetchImage(u string) (image.Image, int, error) {
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", ua)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return img, img.Bounds().Dx(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
