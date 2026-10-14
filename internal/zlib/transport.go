package zlib

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
)

// newStealthTransport 构造带 Chrome TLS 指纹的 Transport：
// Z-Library 的 DiamWall 防护按 TLS ClientHello 指纹（JA3）拦截 Go 默认客户端
// （z-lib.li 直接断连、.sk 返回挑战）。utls 以 Chrome 的指纹完成握手即可通过。
// HTTPS_PROXY 设置时先经代理建立 CONNECT 隧道，再在隧道上做 utls 握手。
func newStealthTransport() *http.Transport {
	return &http.Transport{
		DialTLSContext:        dialUTLS,
		ForceAttemptHTTP2:     false, // Chrome 指纹下用 HTTP/1.1，行为最稳
		MaxIdleConns:          8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// dialUTLS：代理 CONNECT（如有）→ utls Chrome 握手。
func dialUTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	raw, err := dialTCP(ctx, addr)
	if err != nil {
		return nil, err
	}

	tlsConn := utls.UClient(raw, &utls.Config{ServerName: host}, utls.HelloChrome_Auto)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf("TLS 握手失败: %w", err)
	}
	return tlsConn, nil
}

// dialTCP 建立到 addr 的 TCP 连接；配置了 HTTPS_PROXY 时走 CONNECT 隧道。
func dialTCP(ctx context.Context, addr string) (net.Conn, error) {
	proxy := proxyAddr(addr)
	if proxy == nil {
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}

	// 通过代理建立 HTTPS 隧道
	proxyHost := proxy.Host
	if proxy.Port() == "" {
		proxyHost = net.JoinHostPort(proxy.Hostname(), "1080")
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", proxyHost)
	if err != nil {
		return nil, fmt.Errorf("连接代理失败(%s): %w", proxyHost, err)
	}
	req := "CONNECT " + addr + " HTTP/1.1\r\nHost: " + addr + "\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("代理 CONNECT 失败: HTTP %d", resp.StatusCode)
	}
	return conn, nil
}

// proxyAddr 从 HTTPS_PROXY（大小写兼容）解析代理地址；无代理返回 nil。
// 直连本站域名不走代理（保留扩展位：当前全部走代理或全部直连）。
func proxyAddr(_ string) *url.URL {
	for _, k := range []string{"HTTPS_PROXY", "https_proxy"} {
		if v := os.Getenv(k); v != "" {
			u, err := url.Parse(v)
			if err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "socks5") {
				if u.Scheme == "socks5" {
					return nil // socks5 代理此处暂不支持 CONNECT 隧道（http 代理即可满足）
				}
				return u
			}
		}
	}
	return nil
}

// hostOf 返回 URL 的 host:port（默认 443）。
func hostOf(u string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	if !strings.Contains(s, ":") {
		return s + ":443"
	}
	return s
}
