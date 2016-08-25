package meta

// StoreRet
type StoreRet struct {
	Ret int `json:"ret"`
}

// Response
type Response struct {
	Ret    int      `json:"ret"`
	Key    int64    `json:"key"`
	Cookie int32    `json:"cookie"`
	Vid    int32    `json:"vid"`
	Stores []string `json:"stores"`
	MTime  int64    `json:"update_time"`
	Sha1   string   `json:"sha1"`
	Mine   string   `json:"mine"`
}

type ResponseList struct {
	Ret          int
	ResponseList []Response
}

type DirInfo struct {
	Dir     string
	Files   []string
	SubDirs []string
}

type FileSizeInfo struct {
	Filename string
	Filesize int64
}

type ResponseHeadInfo struct {
	Ret          int
	FileSizeInfo FileSizeInfo
	Dir          DirInfo
}
