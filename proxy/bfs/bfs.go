package bfs

import (
	"bfs/libs/errors"
	"bfs/libs/meta"
	"bfs/proxy/conf"
	"bytes"
	"encoding/json"
	"fmt"
	itime "github.com/Terry-Mao/marmot/time"
	log "github.com/golang/glog"
	"github.com/samuel/go-zookeeper/zk"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// api
	_directoryGetApi     = "http://%s/get"
	_directoryUploadApi  = "http://%s/upload"
	_directoryPutInfoApi = "http://%s/putinfo"
	_directoryDelApi     = "http://%s/del"
	_directoryHeadApi    = "http://%s/head"
	_storeGetApi         = "http://%s/get"
	_storeUploadApi      = "http://%s/upload"
	_storeDelApi         = "http://%s/del"
)

var (
	_timer     = itime.NewTimer(1024)
	_transport = &http.Transport{
		Dial: func(netw, addr string) (c net.Conn, err error) {
			if c, err = net.DialTimeout(netw, addr, 2*time.Second); err != nil {
				return nil, err
			}
			return c, nil
		},
		DisableCompression: true,
	}
	_client = &http.Client{
		Transport: _transport,
	}
	_canceler = _transport.CancelRequest
	// random store node
	_rand = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type Bfs struct {
	c       *conf.Config
	xfsaddr []string
	lock    sync.Mutex
	index   int
}

func New(c *conf.Config) (b *Bfs) {
	b = &Bfs{}
	b.c = c
	b.watchZk()
	return
}

func (b *Bfs) watchZk() (err error) {
	// connect to zk
	c, _, err := zk.Connect(b.c.Zookeeper.Addr, time.Duration(b.c.Zookeeper.Timeout)*time.Second)
	if err != nil {
		log.Errorf("connect to zk failed(%v)", err)
		return err
	}

	go func() {
		// watch /directory node
		for {
			nodes, _, ev, err := c.ChildrenW(b.c.Zookeeper.DirectoryRoot)
			if err != nil {
				log.Errorf("watch child failed(%v)", err)
				return
			}
			for _, node := range nodes {
				path := path.Join(b.c.Zookeeper.DirectoryRoot, "/", node)
				data, _, err := c.Get(path)
				if err != nil {
					log.Errorf("zk Get path(%v) failed()")
				}
				b.lock.Lock()
				b.xfsaddr = append(b.xfsaddr, string(data))
				log.Info("directory address:", b.xfsaddr)
				b.lock.Unlock()
			}

			select {
			case <-ev:
			}
			log.Infof("directory node(%s) changed", b.c.Zookeeper.DirectoryRoot)
			b.lock.Lock()
			b.xfsaddr = make([]string, 0)
			b.lock.Unlock()
		}
	}()
	return nil
}

func (b *Bfs) GetXfsAddr() (ip string) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.index = (b.index + 1) % len(b.xfsaddr)
	return b.xfsaddr[b.index]
}

// Get
func (b *Bfs) Get(bucket, filename string, ra *meta.Range) (src io.ReadCloser, ctlen int, mtime int64, sha1, mine string, err error) {
	var (
		i, ix, l  int
		uri       string
		req       *http.Request
		resp      *http.Response
		res       meta.Response
		params    = url.Values{}
		str_range string
	)

	if ra.Start != 0 || ra.End != 0 {
		if ra.End == 0 {
			str_range = fmt.Sprintf("bytes=%d-", ra.Start)
		} else {
			str_range = fmt.Sprintf("bytes=%d-%d", ra.Start, ra.End)
		}
		params.Set("Range", str_range)
	}
	params.Set("bucket", bucket)
	params.Set("filename", filename)
	uri = fmt.Sprintf(_directoryGetApi, b.GetXfsAddr())
	if err = Http("GET", uri, params, nil, &res); err != nil {
		log.Errorf("GET called Http error(%v)", err)
		return
	}
	if res.Ret != errors.RetOK {
		err = errors.Error(res.Ret)
		log.Errorf("http.Get directory res.Ret: %d %s", res.Ret, uri)
		return
	}
	mtime = res.MTime
	sha1 = res.Sha1
	mine = res.Mine
	params = url.Values{}
	l = len(res.Stores)
	ix = _rand.Intn(l)
	for i = 0; i < l; i++ {
		params.Set("key", strconv.FormatInt(res.Key, 10))
		params.Set("cookie", strconv.FormatInt(int64(res.Cookie), 10))
		params.Set("vid", strconv.FormatInt(int64(res.Vid), 10))
		// 将文件的range转化为块的range
		if ra.Start != 0 || ra.End != 0 {
			str_range, _ = ra.ConvertRange(int64(b.c.MaxFileSize))
			params.Set("Range", str_range)
		}
		uri = fmt.Sprintf(_storeGetApi, res.Stores[(ix+i)%l]) + "?" + params.Encode()
		if req, err = http.NewRequest("GET", uri, nil); err != nil {
			continue
		}
		td := _timer.Start(5*time.Second, func() {
			_canceler(req)
		})
		if resp, err = _client.Do(req); err != nil {
			log.Errorf("_client.do(%s) error(%v)", uri, err)
			continue
		}
		td.Stop()
		if resp.StatusCode != http.StatusOK {
			log.Errorf("download http status=%d", resp.StatusCode)
			resp.Body.Close()
			continue
		}
		src = resp.Body
		ctlen = int(resp.ContentLength)
		break
	}
	if err == nil && resp.StatusCode == http.StatusServiceUnavailable {
		err = errors.ErrStoreNotAvailable
	} else if err != nil {
		log.Errorf("read store failed, vid=%v, key=%v, error=%v", res.Vid, res.Key, err)
	}
	return
}

// Upload
func (b *Bfs) Upload(bucket, filename, mine, sha1 string, buf []byte) (err error) {
	var (
		params = url.Values{}
		uri    string
		host   string
		res    meta.Response
		sRet   meta.StoreRet
	)
	params.Set("bucket", bucket)
	params.Set("filename", filename)
	params.Set("mine", mine)
	params.Set("sha1", sha1)
	params.Set("filesize", strconv.FormatInt(int64(len(buf)), 10))
	uri = fmt.Sprintf(_directoryUploadApi, b.GetXfsAddr())
	if err = Http("POST", uri, params, nil, &res); err != nil {
		return
	}
	if res.Ret != errors.RetOK {
		log.Errorf("http.Post directory res.Ret: %d %s", res.Ret, uri)
		err = errors.Error(res.Ret)
		return
	}

	params = url.Values{}
	for _, host = range res.Stores {
		params.Set("key", strconv.FormatInt(res.Key, 10))
		params.Set("cookie", strconv.FormatInt(int64(res.Cookie), 10))
		params.Set("vid", strconv.FormatInt(int64(res.Vid), 10))
		uri = fmt.Sprintf(_storeUploadApi, host)
		if err = Http("POST", uri, params, buf, &sRet); err != nil {
			return
		}
		if sRet.Ret != 1 {
			log.Errorf("http.Post store sRet.Ret: %d  %s %d %d %d", sRet.Ret, uri, res.Key, res.Cookie, res.Vid)
			err = errors.Error(sRet.Ret)
			return
		}
	}
	log.Infof("bfs.upload bucket:%s filename:%s key:%d cookie:%d vid:%d", bucket, filename, res.Key, res.Cookie, res.Vid)
	return
}

// PutInfo
func (b *Bfs) PutInfo(bucket, filename, mime, sha1 string, buf []byte) (err error) {
	var (
		params = url.Values{}
		uri    string
		res    meta.Response
	)
	params.Set("bucket", bucket)
	params.Set("filename", filename)
	params.Set("mine", mime)
	params.Set("sha1", sha1)
	params.Set("filesize", strconv.FormatInt(int64(len(buf)), 10))
	uri = fmt.Sprintf(_directoryPutInfoApi, b.GetXfsAddr())
	if err = Http("POST", uri, params, buf, &res); err != nil {
		return
	}
	if res.Ret != errors.RetOK && res.Ret != errors.RetNeedleExist {
		log.Errorf("http.Post directory res.Ret: %d %s", res.Ret, uri)
		err = errors.ErrInternal
		return
	}

	log.Infof("bfs.putinfo bucket:%s filename:%s", bucket, filename)
	return
}

func (b *Bfs) Head(bucket, filename string) (byte_json []byte, err error) {
	var (
		params = url.Values{}
		uri    string
		res    meta.ResponseHeadInfo
	)
	// set params
	params.Set("bucket", bucket)
	params.Set("filename", filename)
	// set uri
	uri = fmt.Sprintf(_directoryHeadApi, b.GetXfsAddr())
	// http request
	if err = Http("GET", uri, params, nil, &res); err != nil {
		log.Errorf("Head called http error (%v)", err)
		return
	}
	if res.Ret != errors.RetOK {
		if res.Ret == errors.RetNeedleNotExist || res.Ret == errors.RetDirNotExist {
			err = errors.ErrNeedleNotExist
		} else {
			log.Warningf("Head ret from directory failed, uri=%v", uri)
			err = errors.ErrInternal
		}
		return
	}
	if strings.HasSuffix(filename, "/") {
		byte_json, err = json.Marshal(res.Dir)
	} else {
		byte_json, err = json.Marshal(res.FileSizeInfo)
		if err != nil {
			log.Warningf("json failed, res=%v", uri, res)
		}
	}
	return
}

// Delete
func (b *Bfs) Delete(bucket, filename string) (err error) {
	var (
		params   = url.Values{}
		host     string
		uri      string
		res      meta.ResponseList
		sRet     meta.StoreRet
		response meta.Response
	)
	params.Set("bucket", bucket)
	params.Set("filename", filename)
	uri = fmt.Sprintf(_directoryDelApi, b.GetXfsAddr())
	if err = Http("POST", uri, params, nil, &res); err != nil {
		log.Errorf("Delete called Http error(%v)", err)
		return
	}
	if res.Ret != errors.RetOK {
		err = errors.Error(res.Ret)
		log.Errorf("http.Get directory res.Ret: %d %s", res.Ret, uri)
		return
	}

	response = meta.Response{}
	for _, response = range res.ResponseList {
		if response.Ret != errors.RetOK {
			continue
		}

		params = url.Values{}
		for _, host = range response.Stores {
			params.Set("key", strconv.FormatInt(response.Key, 10))
			params.Set("vid", strconv.FormatInt(int64(response.Vid), 10))
			uri = fmt.Sprintf(_storeDelApi, host)
			if err = Http("POST", uri, params, nil, &sRet); err != nil {
				log.Errorf("Update called Http error(%v), uri(%v)", err, uri)
				continue
			}
			if sRet.Ret != 1 {
				log.Errorf("Delete store sRet.Ret: %d  %s", sRet.Ret, uri)
				continue
			}
		}
	}

	return
}

// Ping
func (b *Bfs) Ping() error {
	return nil
}

// Http params
func Http(method, uri string, params url.Values, buf []byte, res interface{}) (err error) {
	var (
		body    []byte
		w       *multipart.Writer
		bw      io.Writer
		bufdata = &bytes.Buffer{}
		req     *http.Request
		resp    *http.Response
		ru      string
		enc     string
		ctype   string
		n       int
	)
	enc = params.Encode()
	if enc != "" {
		ru = uri + "?" + enc
	}
	if method == "GET" {
		if req, err = http.NewRequest("GET", ru, nil); err != nil {
			log.Errorf("http NewRequest error: %v", err)
			return
		}
	} else {
		if buf == nil {
			if req, err = http.NewRequest("POST", uri, strings.NewReader(enc)); err != nil {
				log.Errorf("http NewRequest error: %v", err)
				return
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			w = multipart.NewWriter(bufdata)
			if bw, err = w.CreateFormFile("file", "1.jpg"); err != nil {
				log.Errorf("createformfile error: %v", err)
				return
			}
			if n, err = bw.Write(buf); err != nil {
				log.Warningf("write date to %s failed: len(buf)=%d, n=%d, err=%v", uri, len(buf), n, err)
				return
			}
			for key, _ := range params {
				w.WriteField(key, params.Get(key))
			}
			ctype = w.FormDataContentType()
			if err = w.Close(); err != nil {
				log.Errorf("multipart.Writer Close error: %v", err)
				return
			}
			if req, err = http.NewRequest("POST", uri, bufdata); err != nil {
				log.Errorf("http NewRequest error: %v", err)
				return
			}
			req.Header.Set("Content-Type", ctype)
		}
	}
	td := _timer.Start(10*time.Second, func() {
		log.Errorf("request to directory timeout")
		_canceler(req)
	})
	if resp, err = _client.Do(req); err != nil {
		log.Errorf("_client.Do(%s) error(%v)", ru, err)
		return
	}
	td.Stop()
	defer resp.Body.Close()
	if res == nil {
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Errorf("_client.Do(%s) status: %d", ru, resp.StatusCode)
		return
	}
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		log.Errorf("ioutil.ReadAll(%s) uri(%s) error(%v)", body, ru, err)
		return
	}
	if err = json.Unmarshal(body, res); err != nil {
		log.Errorf("json.Unmarshal(%s) uri(%s) error(%v)", body, ru, err)
	}
	return
}
