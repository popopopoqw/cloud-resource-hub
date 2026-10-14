package server

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"resource-center/internal/pan"
	"resource-center/internal/pan123web"
)

// panWebFS 将 pan123web.Client 适配为 pan.FS——「我的网盘」真实数据源。
// 端点/签名移植自社区逆向实现（alist-123pan-unlock）。
type panWebFS struct {
	c *pan123web.Client

	sizeMu  sync.Mutex
	sizeCch map[int64]sizeEntry // 目录大小缓存（递归遍历昂贵）
}

type sizeEntry struct {
	size     int64
	cachedAt time.Time
}

const dirSizeTTL = time.Hour

// DirSizeCached 带缓存的目录大小（过期后台刷新，先返回旧值）。
func (s *panWebFS) DirSizeCached(id int64) int64 {
	s.sizeMu.Lock()
	defer s.sizeMu.Unlock()
	if s.sizeCch == nil {
		s.sizeCch = map[int64]sizeEntry{}
	}
	if e, ok := s.sizeCch[id]; ok {
		return e.size
	}
	return -1 // 未计算
}

// InvalidateSizes 失效全部目录大小缓存（写操作后调用，保证数据新鲜）。
func (s *panWebFS) InvalidateSizes() {
	s.sizeMu.Lock()
	s.sizeCch = map[int64]sizeEntry{}
	s.sizeMu.Unlock()
}

// DirSizeCompute 计算并缓存目录大小。
func (s *panWebFS) DirSizeCompute(ctx context.Context, id int64) (int64, error) {
	size, err := s.c.DirSize(id)
	if err != nil {
		return 0, err
	}
	s.sizeMu.Lock()
	if s.sizeCch == nil {
		s.sizeCch = map[int64]sizeEntry{}
	}
	s.sizeCch[id] = sizeEntry{size: size, cachedAt: time.Now()}
	s.sizeMu.Unlock()
	return size, nil
}

// toEntry 字段转换。
func toEntry(f pan123web.FileEntry) pan.DirEntry {
	return pan.DirEntry{FileID: f.FileID, Name: f.Filename, IsDir: f.IsDir(), Size: f.Size, UpdateAt: f.UpdateAt}
}

func (s *panWebFS) ListDir(ctx context.Context, dirID int64, page int) (*pan.DirListing, error) {
	const pageSize = 50
	if page < 1 {
		page = 1
	}

	entries, total, err := s.c.ListDirPage(dirID, pageSize, page)
	if err != nil {
		return nil, err
	}
	out := &pan.DirListing{
		DirID: dirID, Page: page,
		PageSize: pageSize, Total: total, HasMore: page*pageSize < total,
		Entries: make([]pan.DirEntry, 0, len(entries)),
	}
	for _, e := range entries {
		out.Entries = append(out.Entries, toEntry(e))
	}
	return out, nil
}

func (s *panWebFS) DirTree(ctx context.Context, root int64) (*pan.DirNode, error) {
	var build func(parent int64, depth int) ([]pan.DirNode, error)
	build = func(parent int64, depth int) ([]pan.DirNode, error) {
		if depth > 6 { // 防深递归失控
			return nil, nil
		}
		entries, err := s.c.ListDir(parent)
		if err != nil {
			return nil, err
		}
		var nodes []pan.DirNode
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			children, err := build(e.FileID, depth+1)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, pan.DirNode{FileID: e.FileID, Name: e.Filename, Dirs: children})
		}
		return nodes, nil
	}
	name := "我的网盘"
	dirs, err := build(root, 0)
	if err != nil {
		return nil, err
	}
	return &pan.DirNode{FileID: root, Name: name, Dirs: dirs}, nil
}

func (s *panWebFS) Mkdir(ctx context.Context, parentID int64, name string) (int64, error) {
	return s.c.MkdirUnder(parentID, name)
}

func (s *panWebFS) Rename(ctx context.Context, fileID int64, name string) error {
	return s.c.Rename(fileID, name)
}

func (s *panWebFS) Move(ctx context.Context, fileIDs []int64, targetID int64) error {
	return s.c.Move(fileIDs, targetID)
}

func (s *panWebFS) Trash(ctx context.Context, fileIDs []int64) error {
	return s.c.Trash(fileIDs)
}

func (s *panWebFS) DownloadURL(ctx context.Context, fileID int64) (string, error) {
	// download_info 需要 fileName/size/type：先从所在目录列出找到该文件
	return s.c.DownloadURL(fileID, "", 0, 0)
}

func (s *panWebFS) Capacity(ctx context.Context) (int64, int64, error) {
	return 0, 0, fmt.Errorf("容量接口未接入（非关键）")
}

var _ pan.FS = (*panWebFS)(nil)
var _ = strconv.Itoa
