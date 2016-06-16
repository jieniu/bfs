package meta

type File struct {
	Filename string `json:"filename"`
	Key      int64  `json:"key"`
	Sha1     string `json:"sha1"`
	Mine     string `json:"mine"`
	Status   int32  `json:"status"`
	Filesize int64  `json:"filesize"`
}
