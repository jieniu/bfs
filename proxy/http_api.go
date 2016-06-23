package main

import (
	"bfs/libs/errors"
	"bfs/libs/meta"
	"bfs/proxy/auth"
	"bfs/proxy/bfs"
	ibucket "bfs/proxy/bucket"
	"bfs/proxy/conf"
	"bfs/proxy/redis_c"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
)

const (
	_httpServerReadTimeout  = 5 * time.Second
	_httpServerWriteTimeout = 2 * time.Second
)

type server struct {
	bfs     *bfs.Bfs
	bucket  *ibucket.Bucket
	auth    *auth.Auth
	c       *conf.Config
	redis_c *redis_c.RedisClient
}

// StartApi init the http module.
func StartApi(c *conf.Config) (err error) {
	var s = &server{}
	s.c = c
	s.bfs = bfs.New(c)
	s.redis_c = redis_c.NewRedisClient()
	if err = redis_c.Init(c); err != nil {
		return
	}
	if s.bucket, err = ibucket.New(); err != nil {
		return
	}
	if s.auth, err = auth.New(c); err != nil {
		return
	}
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", s.do)
		mux.HandleFunc("/ping", s.ping)
		server := &http.Server{
			Addr:         c.HttpAddr,
			Handler:      mux,
			ReadTimeout:  _httpServerReadTimeout,
			WriteTimeout: _httpServerWriteTimeout,
		}
		if err := server.ListenAndServe(); err != nil {
			return
		}
	}()
	return
}

type handler func(*ibucket.Item, string, string, http.ResponseWriter, *http.Request)

func (s *server) do(wr http.ResponseWriter, r *http.Request) {
	var (
		bucket string
		file   string
		token  string
		status int
		err    error
		h      handler
		item   *ibucket.Item
		upload = false
		read   = false
	)
	switch r.Method {
	case "HEAD", "GET":
		// TODO get filesize or download file
		h = s.download
		read = true
	case "PUT", "POST":
		h = s.upload
		upload = true
	case "DELETE":
		h = s.delete
	default:
		http.Error(wr, "", http.StatusMethodNotAllowed)
		return
	}
	if bucket, file, status = s.parseURI(r, upload); status != http.StatusOK {
		http.Error(wr, "", status)
		return
	}
	if item, err = s.bucket.Get(bucket); err != nil {
		log.Errorf("bucket.Get(%s) error(%v)", bucket, err)
		http.Error(wr, "", http.StatusNotFound)
		return
	}
	// item not public must use authorize
	if !item.Public(read) {
		token = r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("Authorization")
		}
		if err = s.auth.Authorize(item, r.Method, bucket, file, token); err != nil {
			log.Errorf("authorize(%s, %s, %s, %s) by item: %v error(%v)", r.Method, bucket, file, token, item, err)
			http.Error(wr, "", http.StatusUnauthorized)
			return
		}
	}
	h(item, bucket, file, wr, r)
	return
}

func httpLog(method, uri string, bucket, file *string, start time.Time, status *int, err *error) {
	log.Infof("%s: %s, bucket: %s, file: %s, time: %f, status: %d, error(%v)",
		method, uri, *bucket, *file, time.Now().Sub(start).Seconds(), *status, *err)
}

// set reponse header.
func setCode(wr http.ResponseWriter, status *int) {
	wr.Header().Set("Code", strconv.Itoa(*status))
}

// parseURI get uri's bucket and filename.
func (s *server) parseURI(r *http.Request, upload bool) (bucket, file string, status int) {
	var b, e int
	status = http.StatusOK
	if s.c.Prefix == "" {
		// uri: /bucket/file...
		//      [1:
		b = 1
	} else {
		// uri: /prefix/bucket/file...
		//             [len(prefix):
		if !strings.HasPrefix(r.URL.Path, s.c.Prefix) {
			log.Errorf("parseURI(%s) error, no prefix: %s", r.URL.Path, s.c.Prefix)
			status = http.StatusBadRequest
			return
		}
		b = len(s.c.Prefix)
	}
	if e = strings.Index(r.URL.Path[b:], "/"); e < 1 {
		bucket = r.URL.Path[b:]
		file = ""
	} else {
		bucket = r.URL.Path[b : b+e]
		file = r.URL.Path[b+e+1:] // skip "/"
	}
	if bucket == "" || (file == "" && !upload) {
		log.Errorf("parseURI(%s) error, bucket: %s or file: %s empty", r.URL.Path, bucket, file)
		status = http.StatusBadRequest
	}
	return
}

// gentRI get uri by bucket and file.
func (s *server) getURI(bucket, file string) (uri string) {
	// http://domain/prefix/bucket/file
	uri = s.c.Domain + path.Join(s.c.Prefix, bucket, file)
	return
}

// download.
func (s *server) download(item *ibucket.Item, bucket, file string, wr http.ResponseWriter, r *http.Request) {
	var (
		err      error
		start    = time.Now()
		status   = http.StatusOK
		ind      int
		method   int
		filename string
		mtime    int64
		ctlen    int
		mine     string
		sha1     string
		src      io.ReadCloser
	)
	defer httpLog("download", r.URL.Path, &bucket, &file, start, &status, &err)

	ind = strings.Index(file, "/")
	if ind < 0 {
		log.Errorf("bad url, %s", file)
		status = http.StatusBadRequest
		http.Error(wr, "", status)
		return
	}
	if method, err = strconv.Atoi(file[:ind]); err != nil {
		status = http.StatusBadRequest
		log.Errorf("str atoi error, %v", err)
		http.Error(wr, "", status)
		return
	}
	filename = file[ind+1:]

	// 0 = get size; 1 = get content
	if method == 0 {
		var f *meta.File
		var result = make(map[string]interface{})
		var byteJson []byte
		if f, err = s.redis_c.GetFile(bucket, filename); err != nil {
			if err == errors.ErrNeedleNotExist {
				status = http.StatusNotFound
				http.Error(wr, "", status)
				return
			} else {
				log.Errorf("redis get meta failed, %v", err)
				status = http.StatusInternalServerError
				http.Error(wr, "", status)
				return
			}
		}
		result["filesize"] = f.Filesize
		if byteJson, err = json.Marshal(result); err != nil {
			log.Error("json.Marshal(%v) failed (%v)", result, err)
			return
		}
		wr.Header().Set("Code", strconv.Itoa(status))
		wr.Header().Set("Content-Type", "application/json;charset=utf-8")
		_, err = wr.Write(byteJson)

	} else if method == 1 {
		if src, ctlen, mtime, sha1, mine, err = s.bfs.Get(bucket, filename); err != nil {
			if err == errors.ErrNeedleNotExist {
				status = http.StatusNotFound
			} else {
				status = http.StatusInternalServerError
			}
			http.Error(wr, "", status)
		} else {
			wr.Header().Set("Content-Type", mine)
			wr.Header().Set("Content-Length", strconv.Itoa(ctlen))
			wr.Header().Set("Server", "xfs")
			wr.Header().Set("Last-Modified", time.Unix(0, mtime).Format(http.TimeFormat))
			wr.Header().Set("Etag", sha1)
			wr.Header().Set("Code", strconv.Itoa(status))
			if src != nil {
				if r.Method == "GET" {
					io.Copy(wr, src)
				}
				src.Close()
			}
		}
	} else {
		status = http.StatusBadRequest
		log.Errorf("method invalid %d", method)
		http.Error(wr, "", status)
		return
	}

	return
}

// ret reponse header.
func retCode(wr http.ResponseWriter, status *int, reterr *error) {
	var (
		result   map[string]interface{}
		byteJson []byte
		err      error
	)
	result = make(map[string]interface{})
	result["ret"] = *status
	result["msg"] = string("ok")
	if *reterr != nil {
		result["msg"] = (*reterr).Error()
	}
	if byteJson, err = json.Marshal(result); err != nil {
		log.Error("json.Marshal(%v) failed (%v)", result, err)
		return
	}
	wr.Header().Set("Code", strconv.Itoa(*status))
	wr.Header().Set("Content-Type", "application/json;charset=utf-8")

	wr.WriteHeader(*status)
	if _, err = wr.Write(byteJson); err != nil {
		log.Errorf("http write error %v", err)
		return
	}
	log.Infof("ret code status %v", *status)
}

// upload upload file.
func (s *server) upload(item *ibucket.Item, bucket, file string, wr http.ResponseWriter, r *http.Request) {
	var (
		ok       bool
		body     []byte
		mine     string
		location string
		sha1sum  string
		ext      string
		sha      [sha1.Size]byte
		err      error
		uerr     errors.Error
		status   = http.StatusOK
		start    = time.Now()
	)
	defer httpLog("upload", r.URL.Path, &bucket, &file, start, &status, &err)
	defer retCode(wr, &status, &err)
	if mine = r.Header.Get("Content-Type"); mine == "" {
		status = http.StatusBadRequest
		return
	}
	if ext = path.Base(mine); ext == "jpeg" {
		ext = "jpg"
	}
	if body, err = ioutil.ReadAll(r.Body); err != nil {
		status = http.StatusBadRequest
		log.Errorf("ioutil.ReadAll(r.Body) error(%s)", err)
		return
	}
	r.Body.Close()
	if len(body) > s.c.MaxFileSize {
		status = http.StatusRequestEntityTooLarge
		return
	}
	sha = sha1.Sum(body)
	sha1sum = hex.EncodeToString(sha[:])
	// if empty filename or endwith "/": dir
	if file == "" || strings.HasSuffix(file, "/") {
		file += sha1sum + "." + ext
	}
	if err = s.bfs.Upload(bucket, file, mine, sha1sum, body); err != nil && err != errors.ErrNeedleExist {
		if uerr, ok = (err).(errors.Error); ok {
			status = int(uerr)
		} else {
			status = http.StatusInternalServerError
		}
		return
	}
	location = s.getURI(bucket, file)
	wr.Header().Set("Location", location)
	wr.Header().Set("ETag", sha1sum)
	return
}

// delete
func (s *server) delete(item *ibucket.Item, bucket, file string, wr http.ResponseWriter, r *http.Request) {
	var (
		ok     bool
		err    error
		uerr   errors.Error
		status = http.StatusOK
		start  = time.Now()
	)
	defer httpLog("delete", r.URL.Path, &bucket, &file, start, &status, &err)
	if err = s.bfs.Delete(bucket, file); err != nil {
		if err == errors.ErrNeedleNotExist {
			status = http.StatusNotFound
			//http.Error(wr, "", status)
		} else {
			if uerr, ok = (err).(errors.Error); ok {
				status = int(uerr)
			} else {
				status = http.StatusInternalServerError
			}
		}
	} else {
		wr.Header().Set("Code", strconv.Itoa(status))
	}
	retCode(wr, &status, &err)
	return
}

// monitorPing sure program now runs correctly, when return http status 200.
func (s *server) ping(wr http.ResponseWriter, r *http.Request) {
	var (
		byteJson []byte
		f        *os.File
		res      = map[string]interface{}{"code": 0}
		err      error
	)
	if f, err = os.Open("/tmp/proxy.ping"); err == nil {
		// ping check
		res["code"] = http.StatusInternalServerError
		f.Close()
	}
	if err = s.bfs.Ping(); err != nil {
		http.Error(wr, "", http.StatusInternalServerError)
		res["code"] = http.StatusInternalServerError
	}
	if byteJson, err = json.Marshal(res); err != nil {
		return
	}
	wr.Header().Set("Content-Type", "application/json;charset=utf-8")
	if _, err = wr.Write(byteJson); err != nil {
		return
	}
	return
}
