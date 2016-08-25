package main

import (
	"bfs/libs/errors"
	"bfs/libs/meta"
	"encoding/json"
	log "github.com/golang/glog"
	"net/http"
	"strconv"
	"time"
)

const (
	_pingOk = 0
)

type server struct {
	d *Directory
}

// StartApi start api http listen.
func StartApi(addr string, d *Directory) {
	var s = &server{d: d}
	go func() {
		var (
			err      error
			serveMux = http.NewServeMux()
		)
		serveMux.HandleFunc("/get", s.get)
		serveMux.HandleFunc("/upload", s.upload)
		serveMux.HandleFunc("/putinfo", s.putInfo)
		serveMux.HandleFunc("/del", s.del)
		serveMux.HandleFunc("/head", s.head)
		serveMux.HandleFunc("/ping", s.ping)
		if err = http.ListenAndServe(addr, serveMux); err != nil {
			log.Errorf("http.ListenAndServe(\"%s\") error(%v)", addr, err)
			return
		}
	}()
	return
}

func (s *server) get(wr http.ResponseWriter, r *http.Request) {
	var (
		ok        bool
		bucket    string
		filename  string
		res       meta.Response
		n         *meta.Needle
		f         *meta.File
		uerr      errors.Error
		err       error
		params    = r.URL.Query()
		tr        = &meta.Range{}
		str_range string
		status    int
	)
	if r.Method != "GET" {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if bucket = r.FormValue("bucket"); bucket == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if filename = r.FormValue("filename"); filename == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	defer HttpGetWriter(r, wr, time.Now(), &res)
	str_range = params.Get("Range")
	if err, status = tr.GetRange(str_range); err != nil {
		log.Errorf("get range error str_range=%s, err=%v", str_range, err)
		res.Ret = status
		return
	}
	if n, f, res.Stores, err = s.d.GetStores(bucket, filename, tr); err != nil {
		log.Errorf("GetStores() error(%v)", err)
		if uerr, ok = err.(errors.Error); ok {
			res.Ret = int(uerr)
		} else {
			res.Ret = errors.RetInternalErr
		}
		return
	}
	res.Ret = errors.RetOK
	res.Key = n.Key
	res.Cookie = n.Cookie
	res.Vid = n.Vid
	res.Mine = f.Mine
	if f.MTime != 0 {
		res.MTime = f.MTime
	} else {
		res.MTime = n.MTime
	}
	res.Sha1 = f.Sha1
	return
}

func (s *server) head(wr http.ResponseWriter, r *http.Request) {

	var (
		filename string
		err      error
		resp     *meta.ResponseHeadInfo
		dirinfo  meta.DirInfo
		f        *meta.File
		bucket   string
	)
	resp = new(meta.ResponseHeadInfo)

	if r.Method != "GET" {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if bucket = r.FormValue("bucket"); bucket == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	// filename
	if filename = r.FormValue("filename"); filename == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if strings.HasSuffix(filename, "/") {
		// dir
		dirinfo, err = s.d.redis_c.GetDirInfo(bucket, filename)
		if err == errors.ErrDirNotExist {
			resp.Ret = errors.RetDirNotExist
		} else if err != nil {
			resp.Ret = errors.RetInternalErr
		} else {
			resp.Ret = errors.RetOK
			resp.Dir = dirinfo
		}
	} else {
		// file
		_, f, err = s.d.redis_c.Get(bucket, filename)
		if err == errors.ErrNeedleNotExist {
			resp.Ret = errors.RetNeedleNotExist
		} else if err != nil {
			resp.Ret = errors.RetInternalErr
		} else {
			resp.Ret = errors.RetOK
			resp.FileSizeInfo.Filename = f.Filename
			resp.FileSizeInfo.Filesize = f.Filesize
		}
	}
	HttpHeadWriter(r, wr, time.Now(), resp)
}

func (s *server) upload(wr http.ResponseWriter, r *http.Request) {
	var (
		err    error
		n      *meta.Needle
		f      *meta.File
		bucket string
		res    meta.Response
		ok     bool
		uerr   errors.Error
	)
	if r.Method != "POST" {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	f = new(meta.File)
	if bucket = r.FormValue("bucket"); bucket == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if f.Filename = r.FormValue("filename"); f.Filename == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if f.Sha1 = r.FormValue("sha1"); f.Sha1 == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if f.Mine = r.FormValue("mine"); f.Mine == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if f.Filesize, err = strconv.ParseInt(r.FormValue("filesize"), 10, 64); err != nil {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if f.Filesize > s.d.config.MaxFileSize {
		log.Errorf("filesize is too large %d, maxfilesize=%d", f.Filesize, s.d.config.MaxFileSize)
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	defer HttpUploadWriter(r, wr, time.Now(), &res)

	res.Ret = errors.RetOK
	if n, res.Stores, err = s.d.UploadStores(bucket, f); err != nil {
		log.Errorf("UploadStores() error(%v)", err)
		if uerr, ok = err.(errors.Error); ok {
			res.Ret = int(uerr)
		} else {
			res.Ret = errors.RetInternalErr
		}
		return
	}
	res.Key = n.Key
	res.Cookie = n.Cookie
	res.Vid = n.Vid
	return
}

func (s *server) putInfo(wr http.ResponseWriter, r *http.Request) {
	var (
		err    error
		f      *meta.File
		fm     *meta.FileMin
		bucket string
		res    meta.Response
		file   multipart.File
		buf    bytes.Buffer
		body   string
	)
	if r.Method != "POST" {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	f = new(meta.File)
	if bucket = r.FormValue("bucket"); bucket == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if f.Filename = r.FormValue("filename"); f.Filename == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if f.Sha1 = r.FormValue("sha1"); f.Sha1 == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if f.Mine = r.FormValue("mine"); f.Mine == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if file, _, err = r.FormFile("file"); err != nil {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	defer file.Close()
	defer HttpUploadWriter(r, wr, time.Now(), &res)

	buf.ReadFrom(file)
	body = buf.String()
	if err = json.Unmarshal([]byte(body), &fm); err != nil {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	f.Filesize = fm.Filesize
	f.Chunks = fm.Chunks

	res.Ret = errors.RetOK
	if err = s.d.redis_c.PutInfo(bucket, f); err != nil {
		res.Ret = errors.RetInternalErr
	}

	return
}

func (s *server) del(wr http.ResponseWriter, r *http.Request) {
	var (
		bucket   string
		filename string
		reslist  *meta.ResponseList
	)

	if r.Method != "POST" {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if bucket = r.FormValue("bucket"); bucket == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	if filename = r.FormValue("filename"); filename == "" {
		http.Error(wr, "bad request", http.StatusBadRequest)
		return
	}
	reslist = new(meta.ResponseList)

	defer HttpWriter(r, wr, time.Now(), reslist)
	if strings.HasPrefix(filename, "/") == false {
		filename = "/" + filename
	}

	reslist.Ret = errors.RetOK
	if strings.HasSuffix(filename, "/") == true {
		err := s.d.DelDirectory(bucket, filename, reslist)
		if err == errors.ErrDirNotExist {
			reslist.Ret = errors.RetDirNotExist
		} else if err != nil {
			reslist.Ret = errors.RetInternalErr
			log.Errorf("http del error, path: %s, err: %s", r.URL.Path, err)
		}
	} else {
		err := s.d.DelFile(bucket, filename, reslist)
		if err == errors.ErrNeedleNotExist {
			reslist.Ret = errors.RetNeedleNotExist
		} else if err != nil {
			reslist.Ret = errors.RetInternalErr
		}
	}

	return
}

func (s *server) ping(wr http.ResponseWriter, r *http.Request) {
	var (
		byteJson []byte
		res      = map[string]interface{}{"code": _pingOk}
		err      error
	)
	if byteJson, err = json.Marshal(res); err != nil {
		log.Errorf("json.Marshal(\"%v\") failed (%v)", res, err)
		return
	}
	wr.Header().Set("Content-Type", "application/json;charset=utf-8")
	if _, err = wr.Write(byteJson); err != nil {
		log.Errorf("HttpWriter Write error(%v)", err)
	}
	return
}
