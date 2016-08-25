package meta

// 块信息
type ChunkInfo struct {
	Filename string `json:"filename"` // chunk name，eg. /dir/filename/0000
	Offset   int64  `json:"offset"`   // 在大文件中的偏移
	Size     int64  `json:"size"`     // 块大小
}

type File struct {
	Filename string `json:"filename"`
	Key      int64  `json:"key"`
	Sha1     string `json:"sha1"`
	Mine     string `json:"mine"`
	Status   int32  `json:"status"`
	Filesize int64  `json:"filesize"`
	MTime    int64  `json:"update_time"`
	Chunks   []ChunkInfo
}

type FileMin struct {
	Filename  string `json:"filename"`
	Filesize  int64  `json:"filesize"`
	Chucksize int64  `json:"chucksize"`
	Chunks    []ChunkInfo
}
