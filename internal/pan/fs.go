package pan

import "context"

// DirEntry 目录条目（文件或文件夹）。
type DirEntry struct {
	FileID   int64  `json:"file_id"`
	Name     string `json:"name"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	UpdateAt string `json:"update_at,omitempty"`
}

// DirListing 目录内容（分页）。
type DirListing struct {
	DirID    int64      `json:"dir_id"`
	ParentID int64      `json:"parent_id"`
	Entries  []DirEntry `json:"entries"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Total    int        `json:"total"`
	HasMore  bool       `json:"has_more"`
}

// DirNode 目录树节点（/drive/tree 用）。
type DirNode struct {
	FileID int64     `json:"file_id"`
	Name   string    `json:"name"`
	Dirs   []DirNode `json:"dirs,omitempty"`
}

// FS 网盘文件管理接口（与 Provider 分离：转存是 Provider，文件管理是 FS）。
// 实现方：Mock（演示文件树）、Pan123（开放平台）。
// 后期接入夸克等网盘时，其 Provider 可选实现本接口，「我的网盘」自动支持。
type FS interface {
	// ListDir 列出目录内容。dirID=0 为根目录；page 从 1 起。
	ListDir(ctx context.Context, dirID int64, page int) (*DirListing, error)
	// DirTree 返回目录树（仅文件夹），root 为起始目录（0=根）。
	DirTree(ctx context.Context, root int64) (*DirNode, error)
	// Mkdir 新建文件夹，返回新目录 ID。
	Mkdir(ctx context.Context, parentID int64, name string) (int64, error)
	// Rename 重命名。
	Rename(ctx context.Context, fileID int64, name string) error
	// Move 移动文件/文件夹到目标目录。
	Move(ctx context.Context, fileIDs []int64, targetID int64) error
	// Trash 移入网盘回收站（可恢复，非彻底删除）。
	Trash(ctx context.Context, fileIDs []int64) error
	// DownloadURL 取文件下载直链（浏览器 302 用）。
	DownloadURL(ctx context.Context, fileID int64) (string, error)
	// Capacity 已用/总容量（字节）。
	Capacity(ctx context.Context) (used, total int64, err error)
}
