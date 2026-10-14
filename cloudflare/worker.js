// Cloudflare Worker 边缘反代示例（配合 wrangler.example.toml 使用）：
// 静态资源边缘缓存 + API 透传 + 简单速率限制。占位实现，按需扩展。
export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const isStatic = url.pathname.startsWith("/static/");

    // 简单限流：按 IP 每分钟 60 次（演示用，生产建议用 Cloudflare WAF / Rate Limiting）
    const bucket = env.RATE_LIMIT?.get(url.pathname, { ignoreMethod: true });
    if (bucket && bucket.success && bucket.result && bucket.result.remaining === 0) {
      return new Response(JSON.stringify({ code: 429, msg: "请求过于频繁" }), {
        status: 429,
        headers: { "Content-Type": "application/json" },
      });
    }

    // 静态资源：一年边缘缓存（文件名带 hash 后很适合）
    if (isStatic) {
      const cache = caches.default;
      const cached = await cache.match(request);
      if (cached) return cached;
      const resp = await fetch(new Request(`${env.ORIGIN}${url.pathname}`, request));
      const clone = new Response(resp.body, resp);
      clone.headers.set("Cache-Control", "public, max-age=86400, s-maxage=604800");
      ctx.waitUntil(cache.put(request, clone.clone()));
      return clone;
    }

    // API：直接回源
    return fetch(new Request(`${env.ORIGIN}${url.pathname}${url.search}`, request));
  },
};
