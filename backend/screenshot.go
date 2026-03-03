package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	screenshotViewportWidth  = 1440
	screenshotViewportHeight = 900
	screenshotQuality        = 80 // JPEG quality (0-100)
	screenshotTimeout        = 45 * time.Second
	screenshotRetryInterval  = 24 * time.Hour
	warpProxy                = "127.0.0.1:40000"
)

// baseScreenshotOpts returns the common Chrome flags for screenshot capture.
func baseScreenshotOpts() []chromedp.ExecAllocatorOption {
	return []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", "new"), // new headless — indistinguishable from real Chrome
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-component-extensions-with-background-pages", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"),
		chromedp.WindowSize(screenshotViewportWidth, screenshotViewportHeight),
	}
}

// waitForCloudflareChallenge polls the page title to detect Cloudflare challenge
// pages ("Just a moment...") and waits for them to auto-resolve. Returns once
// the title changes or maxWait is reached.
func waitForCloudflareChallenge(ctx context.Context, maxWait time.Duration) {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		var title string
		if err := chromedp.Title(&title).Do(ctx); err != nil {
			return
		}
		titleLower := strings.ToLower(title)
		isCF := strings.Contains(titleLower, "just a moment") ||
			strings.Contains(titleLower, "attention required") ||
			strings.Contains(titleLower, "checking your browser")
		if !isCF {
			return // Challenge resolved (or was never present)
		}
		time.Sleep(2 * time.Second)
	}
}

// runScreenshot launches a headless Chrome with the given options, navigates to
// the URL, and returns the JPEG screenshot bytes.
func runScreenshot(url string, opts []chromedp.ExecAllocatorOption) ([]byte, error) {
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, screenshotTimeout)
	defer cancel()

	var buf []byte
	err := chromedp.Run(ctx,
		// Inject script before any page JS runs to hide automation signals
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(`
				Object.defineProperty(navigator, 'webdriver', { get: () => false });
				Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4, 5] });
				Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
				window.chrome = { runtime: {} };
			`).Do(ctx)
			return err
		}),
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		// Wait for Cloudflare challenge to resolve (up to 20s), then render
		chromedp.ActionFunc(func(ctx context.Context) error {
			waitForCloudflareChallenge(ctx, 20*time.Second)
			return nil
		}),
		chromedp.Sleep(3*time.Second),
		chromedp.FullScreenshot(&buf, screenshotQuality),
	)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// isProxyError returns true if the error is a SOCKS/proxy connection failure.
func isProxyError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "SOCKS") || strings.Contains(s, "proxy") || strings.Contains(s, "ERR_PROXY")
}

// resolveScreenshotURL does a lightweight HTTP GET to follow redirects and
// verify the domain is reachable. Returns the final URL after all redirects.
// This avoids wasting 45s launching Chrome against an unreachable domain, and
// ensures we screenshot the actual destination (e.g. anthropic.com → www.anthropic.com).
func resolveScreenshotURL(domain string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("GET", "https://"+domain, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("domain %s unreachable: %w", domain, err)
	}
	resp.Body.Close()

	finalURL := resp.Request.URL.String()
	return finalURL, nil
}

// captureScreenshot takes a JPEG screenshot of the given domain's homepage.
// Resolves redirects first, then tries through WARP proxy; falls back to direct.
func captureScreenshot(domain string) ([]byte, error) {
	// Resolve the final URL (follows redirects, verifies reachability)
	url, err := resolveScreenshotURL(domain)
	if err != nil {
		return nil, fmt.Errorf("screenshot pre-check failed for %s: %w", domain, err)
	}
	if url != "https://"+domain && url != "https://"+domain+"/" {
		log.Printf("Screenshot for %s: resolved to %s", domain, url)
	}

	opts := baseScreenshotOpts()

	// Route through Cloudflare WARP if the proxy is available.
	useProxy := false
	if conn, err := net.DialTimeout("tcp", warpProxy, 2*time.Second); err == nil {
		conn.Close()
		useProxy = true
	}

	if useProxy {
		proxyOpts := append(append([]chromedp.ExecAllocatorOption{}, opts...), chromedp.ProxyServer("socks5://"+warpProxy))
		log.Printf("Screenshot for %s: routing through WARP proxy", domain)
		buf, err := runScreenshot(url, proxyOpts)
		if err == nil {
			return buf, nil
		}
		if isProxyError(err) {
			log.Printf("Screenshot for %s: WARP proxy failed (%v), retrying direct", domain, err)
		} else {
			return nil, fmt.Errorf("screenshot capture failed for %s: %w", domain, err)
		}
	}

	// Direct (no proxy) — either WARP unavailable or proxy connection failed
	log.Printf("Screenshot for %s: direct connection", domain)
	buf, err := runScreenshot(url, opts)
	if err != nil {
		return nil, fmt.Errorf("screenshot capture failed for %s: %w", domain, err)
	}
	return buf, nil
}

// captureBrandScreenshot captures and stores a screenshot for a domain.
// tenantID tracks which tenant triggered the capture (provenance).
func captureBrandScreenshot(mongoDB *MongoDB, domain, tenantID string) {
	log.Printf("Capturing screenshot for %s...", domain)

	imgData, err := captureScreenshot(domain)
	if err != nil {
		log.Printf("Screenshot capture failed for %s: %v", domain, err)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		filter := bson.M{"domain": domain}
		update := bson.M{
			"$set": bson.M{
				"error":      err.Error(),
				"capturedAt": time.Now(),
				"sizeBytes":  0,
				"tenantId":   tenantID,
			},
			"$setOnInsert": bson.M{
				"domain":      domain,
				"contentType": "image/jpeg",
				"width":       screenshotViewportWidth,
				"height":      screenshotViewportHeight,
			},
		}
		mongoDB.BrandScreenshots().UpdateOne(ctx, filter, update,
			options.Update().SetUpsert(true))
		return
	}

	log.Printf("Screenshot captured for %s (%d bytes)", domain, len(imgData))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	doc := BrandScreenshot{
		TenantID:    tenantID,
		Domain:      domain,
		ImageData:   imgData,
		ContentType: "image/jpeg",
		Width:       screenshotViewportWidth,
		Height:      screenshotViewportHeight,
		SizeBytes:   len(imgData),
		CapturedAt:  time.Now(),
	}

	filter := bson.M{"domain": domain}
	replaceOpts := options.Replace().SetUpsert(true)
	if _, err := mongoDB.BrandScreenshots().ReplaceOne(ctx, filter, doc, replaceOpts); err != nil {
		log.Printf("Failed to store screenshot for %s: %v", domain, err)
	}
}

// ensurePopularScreenshots checks all popular domains and captures screenshots
// for any that are missing. Designed to run as a goroutine at startup.
func ensurePopularScreenshots(mongoDB *MongoDB) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cursor, err := mongoDB.DomainShares().Find(ctx, bson.M{"visibility": "popular"})
	if err != nil {
		log.Printf("ensurePopularScreenshots: failed to query popular domains: %v", err)
		return
	}
	defer cursor.Close(ctx)

	var shares []DomainShare
	if err := cursor.All(ctx, &shares); err != nil {
		log.Printf("ensurePopularScreenshots: failed to decode shares: %v", err)
		return
	}

	if len(shares) == 0 {
		return
	}

	domains := make([]string, len(shares))
	for i, s := range shares {
		domains[i] = s.Domain
	}

	existingCursor, err := mongoDB.BrandScreenshots().Find(ctx, bson.M{
		"domain": bson.M{"$in": domains},
	}, options.Find().SetProjection(bson.M{"domain": 1, "error": 1, "capturedAt": 1, "sizeBytes": 1}))
	if err != nil {
		log.Printf("ensurePopularScreenshots: failed to query existing screenshots: %v", err)
		return
	}
	var existing []BrandScreenshot
	existingCursor.All(ctx, &existing)
	existingCursor.Close(ctx)

	existingMap := make(map[string]BrandScreenshot)
	for _, s := range existing {
		existingMap[s.Domain] = s
	}

	now := time.Now()
	var needCapture []string
	for _, d := range domains {
		ss, found := existingMap[d]
		if !found {
			needCapture = append(needCapture, d)
		} else if ss.SizeBytes == 0 && ss.Error != "" {
			// Retry failed captures after cooldown
			if now.Sub(ss.CapturedAt) > screenshotRetryInterval {
				needCapture = append(needCapture, d)
			}
		} else if ss.SizeBytes > 0 && ss.SizeBytes < 5000 {
			// Suspiciously small — likely a blank or error page, recapture
			needCapture = append(needCapture, d)
		}
	}

	if len(needCapture) == 0 {
		log.Printf("ensurePopularScreenshots: all %d popular domains have screenshots", len(shares))
		return
	}

	// Build domain→tenantID map from shares for provenance tracking
	shareTenantMap := make(map[string]string)
	for _, s := range shares {
		shareTenantMap[s.Domain] = s.TenantID
	}

	log.Printf("ensurePopularScreenshots: capturing screenshots for %d domains", len(needCapture))
	for _, domain := range needCapture {
		captureBrandScreenshot(mongoDB, domain, shareTenantMap[domain])
	}
}
