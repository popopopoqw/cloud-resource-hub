package respool

import "testing"

func TestExtractCoverURLFromFirstPost(t *testing.T) {
	html := `
<img src="view/img/logo.png">
<div class="message break-all" isfirst="1">
  <img src="upload/avatar/000/1.png">
  正文<img data-src="https://image.tmdb.org/t/p/w500/poster.jpg" alt="海报">
</div>
<div class="haya-post-like"></div>
<div class="message"><img src="https://example.com/reply.jpg"></div>`
	got := extractCoverURL(html, "https://pan1.me")
	if got != "https://image.tmdb.org/t/p/w500/poster.jpg" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractCoverURLResolvesRelativeImage(t *testing.T) {
	html := `<div class="message" isfirst='1'><img src="upload/attach/poster.webp"></div><div class="haya-post-like"></div>`
	got := extractCoverURL(html, "https://pan1.me")
	if got != "https://pan1.me/upload/attach/poster.webp" {
		t.Fatalf("got %q", got)
	}
}
