package pan123web

import (
	"encoding/json"
	"testing"
)

func TestBuildCopySaveBodyUsesTargetFolderForEveryFile(t *testing.T) {
	body, count, err := buildCopySaveBody("share-key", "abcd", 9876, []ShareFile{
		{FileID: 11, Filename: "a.mp4"},
		{FileID: 22, Filename: "folder", Type: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count=%d, want 2", count)
	}
	var payload struct {
		ParentFileID int64 `json:"parentFileId"`
		FileList     []struct {
			FileID       int64 `json:"fileId"`
			ParentFileID int64 `json:"parentFileId"`
		} `json:"file_list"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ParentFileID != 9876 || len(payload.FileList) != 2 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	for _, file := range payload.FileList {
		if file.ParentFileID != 9876 {
			t.Fatalf("file %d targets %d, want 9876", file.FileID, file.ParentFileID)
		}
	}
}

func TestPlanTargetPath(t *testing.T) {
	tests := []struct {
		name, title, mediaType, want string
	}{
		{"韩剧", "韩国电视剧 我的解放日志", "tv", "/影视合集/韩剧"},
		{"国产剧", "[大陆] 兰香如故", "tv", "/影视合集/国产剧"},
		{"无地区电视剧默认未分类", "雍正王朝 全集", "tv", "/影视合集/未分类"},
		{"美剧", "HBO 美剧 权力的游戏", "tv", "/影视合集/美剧"},
		{"电影", "流浪地球", "movie", "/影视合集/电影"},
		{"综艺", "奔跑吧", "variety", "/影视合集/综艺"},
		{"动漫", "海贼王", "anime", "/影视合集/动漫"},
		{"纪录片", "地球脉动", "documentary", "/影视合集/纪录片"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlanTargetPath(tt.title, "2026", tt.mediaType); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
