package respool

import "testing"

func TestParseThreadPageExcludesSidebarAndReadsPagination(t *testing.T) {
	html := `
<a href="?forum-38.htm"><span>主题:<b>8</b></span></a>
<a href="?forum-48.htm"><span>主题:<b>7174</b></span></a>
<ul class="list-unstyled threadlist mb-0">
  <li><a href="?thread-101.htm">电视剧资源一</a></li>
  <li><a href="?thread-102.htm">电视剧资源二</a></li>
</ul>
<ul class="pagination">
  <li><a href="?forum-48-1.htm">1</a></li>
  <li><a href="?forum-48-359.htm">...359</a></li>
</ul>
<div class="hot"><a href="?thread-999.htm">版块热门帖子</a></div>`

	page, err := parseThreadPage(html, Source{Base: "https://pan1.me"}, "48", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Threads) != 2 {
		t.Fatalf("got %d main threads, want 2", len(page.Threads))
	}
	for _, thread := range page.Threads {
		if thread.ID == "999" {
			t.Fatal("sidebar hot thread must not be included")
		}
	}
	if page.PageSize != 2 || page.Total != 7174 || page.TotalPages != 359 {
		t.Fatalf("unexpected page metadata: %+v", page)
	}
}
