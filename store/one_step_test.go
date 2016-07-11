package main

import (
    "bfs/store/conf"
    "bfs/store/needle"
    "bfs/store/zk"
    "encoding/json"
    "io"
    "io/ioutil"
    "net/http"
    "mime/multipart"
    "crypto/md5"
    "encoding/hex"
    "strings"
    "bytes"
    "strconv"
    "time"
    "os"
    "fmt"
    "testing"
)


type httpRet struct {
    Ret int `json:"ret"`
}


var (
    vid                    = 1
    testDir                = "/data5/yunwen01/"

    ip                     = "10.10.191.9"
    zkPort                 = "2381"
    adminPort              = "16064"
    apiPort                = "16062"

    adminHost              = ip + ":" + adminPort
    adminUriAddFreeVolume  = "http://" + adminHost + "/add_free_volume"
    adminUriAddVolume      = "http://" + adminHost + "/add_volume"
    adminUriCompactVolume  = "http://" + adminHost + "/compact_volume"

    apiHost                = ip + ":" + apiPort
    apiUriUpload           = "http://" + apiHost + "/upload"
    apiUriUploads          = "http://" + apiHost + "/uploads"
    apiUriGet              = "http://" + apiHost + "/get?vid=%d&key=%d&cookie=%d"
    apiUriDel              = "http://" + apiHost + "/del"

    volumeNum              = 4
    testFileName           = "./test/%d.jpg"
    dlFileName             = testDir + "dl_%d.jpg"

    oneStepConf = &conf.Config {
	NeedleMaxSize: 4 * 1024 * 1024,
	BlockMaxSize:  needle.Size(4 * 1024 * 1024),
	BatchMaxNum:   16,
	Zookeeper: &conf.Zookeeper{
	    Root:     "/rack",
	    Rack:     "rack-a",
	    ServerId: "store-a",
	    Addrs:    []string{ip + ":" + zkPort},
	    Timeout:  conf.Duration{time.Second},
	},
	Store: &conf.Store{
	    VolumeIndex:     testDir + "volume.idx",
	    FreeVolumeIndex: testDir + "free_volume.idx",
	},
	Volume: &conf.Volume{
	    SyncDelete:      10,
	    SyncDeleteDelay: conf.Duration{10 * time.Second},
	},
	Block: &conf.Block{
	    BufferSize:    4 * 1024 * 1024,
	    SyncWrite:     1024,
	    Syncfilerange: true,
	},
	Index: &conf.Index{
	    BufferSize:    4 * 1024 * 1024,
	    MergeDelay:    conf.Duration{10 * time.Second},
	    MergeWrite:    5,
	    RingBuffer:    100,
	    SyncWrite:     10,
	    Syncfilerange: true,
	},
	Limit: &conf.Limit {
	    Read:          &conf.Rate{150.0, 50},
	    Write:         &conf.Rate{150.0, 50},
	    Delete:        &conf.Rate{150.0, 50},
	},
    }
)


func GetMd5OfFile(filename string) (md5Str string, phase string, err error) {
    var (
	buf []byte
    )
    if buf, err = ioutil.ReadFile(filename); err != nil {
	phase = "Open File"
	return
    }
    md5Ctx := md5.New()
    md5Ctx.Write(buf)
    cipherStr := md5Ctx.Sum(nil)
    md5Str = hex.EncodeToString(cipherStr)

    phase = "Finished"
    return
}


func clearDiskFiles() {
    // remove index files
    os.Remove(oneStepConf.Store.VolumeIndex)
    os.Remove(oneStepConf.Store.FreeVolumeIndex)

    // remove superblock files
    for i := 0; i <= volumeNum; i++ {
	os.Remove(fmt.Sprintf("%s_free_block_%d", testDir, i))
	os.Remove(fmt.Sprintf("%s_free_block_%d.idx", testDir, i))
	os.Remove(fmt.Sprintf("%s1_%d", testDir, i))
	os.Remove(fmt.Sprintf("%s1_%d.idx", testDir, i))
    }

    // remove files from /get request
    for i := 1; i <= 10; i++ {
	os.Remove(fmt.Sprintf(dlFileName, i))
    }
}


func httpGetAndResp(uri string, body io.Writer) (phase string, err error) {
    var (
	resp  *http.Response
	buf   []byte

    )

    if resp, err = http.Get(uri); err != nil {
	phase = "Get"
	return
    }
    defer resp.Body.Close()

    if buf, err = ioutil.ReadAll(resp.Body); err != nil {
	phase = "Recv"
	return
    }
    if _, err = body.Write(buf); err != nil {
	phase = "Write"
	return
    }

    return
}


func httpPostAndResp(uri string, contentType string, body io.Reader) (phase string, err error) {
    var (
	resp  *http.Response
	buf   []byte
	tr    = &httpRet{}
    )

    if resp, err = http.Post(uri, contentType, body); err != nil {
	phase = "Post"
	return
    }
    defer resp.Body.Close()

    if buf, err = ioutil.ReadAll(resp.Body); err != nil {
	phase = "Recv"
	return
    }

    if strings.Contains(resp.Header["Content-Type"][0], "application/json") {
	if err = json.Unmarshal(buf, tr); err != nil {
	    phase = "Json Unmarshal"
	    return
	}
	if tr.Ret != 1 {
	    phase = "Ret=" + strconv.Itoa(tr.Ret)
	    return
	}
    } else {
	// TODO: set error
    }

    phase = "Finished"
    return
}


func httpPostMultipartAndResp(uri string, values map[string][]string) (phase string, err error) {
    var (
	client  http.Client
	req     *http.Request
	resp    *http.Response
	w       *multipart.Writer
	f       *os.File
	bw      io.Writer
	body    []byte
	buf     = &bytes.Buffer{}
	tr      = &httpRet{}
    )

    buf.Reset()
    w = multipart.NewWriter(buf)

    for k, v := range values {
	for _, val := range v {
	    if k == "file" {
		if bw, err = w.CreateFormFile("file", val); err != nil {
		    phase = "CreateFormFile"
		    return
		}
		if f, err = os.Open(val); err != nil {
		    phase = "Open File"
		    return
		}
		defer f.Close()
		if _, err = io.Copy(bw, f); err != nil {
		    phase = "Copy File"
		    return
		}
	    } else {
		if err = w.WriteField(k, val); err != nil {
		    phase = "WriteField"
		    return
		}
	    }
	}
    }
    w.Close()

    if req, err = http.NewRequest("POST", uri, buf); err != nil {
	phase = "NewRequest"
	return
    }

    req.Header.Set("Content-Type", w.FormDataContentType())
    if resp, err = client.Do(req); err != nil {
	phase = "Post"
	return
    }
    defer resp.Body.Close()

    if body, err = ioutil.ReadAll(resp.Body); err != nil {
	phase = "Recv"
	return
    }

    if err = json.Unmarshal(body, tr); err != nil {
	phase = "Json Unmarshal"
	return
    }

    if tr.Ret != 1 {
	phase = "Ret=" + strconv.Itoa(tr.Ret)
	return
    }

    phase = "Finished"
    return
}


func TestOneStep(t *testing.T) {
    var (
	svr     *Server
	s       *Store
	z       *zk.Zookeeper
	phase   string
	err     error
	uri     string
	buf     = &bytes.Buffer{}
	values  map[string][]string
    )

    clearDiskFiles()
    defer clearDiskFiles()

    // connect to zookeeper
    if z, err = zk.NewZookeeper(oneStepConf); err != nil {
	t.Errorf("NewZookeeper() error(%v)", err)
	t.FailNow()
    }
    defer z.Close()

    // clear volume in zookeeper
    for i := 1; i <= volumeNum; i++ {
	z.DelVolume(int32(i))
	defer z.DelVolume(int32(i))
    }

    // init store
    if s, err = NewStore(oneStepConf); err != nil {
	t.Errorf("NewStore() error(%v)", err)
	t.FailNow()
    }
    //defer s.Close()

    // start http server
    svr = NewServer(s, oneStepConf)
    StartAdmin(adminHost, svr)
    StartApi(apiHost, svr)
    time.Sleep(1 * time.Second)

    // request admin /add_free_volume
    buf.Reset()
    buf.WriteString(fmt.Sprintf("n=%d&bdir=%s&idir=%s", volumeNum, testDir,
            testDir))
    if phase, err = httpPostAndResp(adminUriAddFreeVolume,
            "application/x-www-form-urlencoded", buf); err != nil {
	t.Errorf("%s: %s error(%v)", "/add_free_volume", phase, err)
	t.FailNow()
    }

    // request admin /add_volume
    buf.Reset()
    buf.WriteString(fmt.Sprintf("vid=%d", vid))
    if phase, err = httpPostAndResp(adminUriAddVolume,
            "application/x-www-form-urlencoded", buf); err != nil {
	t.Errorf("%s %s error(%v)", "/add_volume", phase, err)
	t.FailNow()
    }

    // request api /upload
    for i := 1; i <= 3; i++ {
	values = make(map[string][]string)
	values["file"] = []string {fmt.Sprintf(testFileName, i)}
	values["vid"] = []string {strconv.Itoa(vid)}
	values["key"] = []string {strconv.Itoa(i)}
	values["cookie"] = []string {strconv.Itoa(i)}
	if phase, err = httpPostMultipartAndResp(apiUriUpload, values);
	        err != nil {
	    t.Errorf("%s: %s error(%v)", "/upload", phase, err)
	    t.FailNow()
	}
	values = nil
    }

    // request api /uploads
    values = make(map[string][]string)
    values["vid"] = []string {strconv.Itoa(vid)}
    values["file"] = []string {}
    values["keys"] = []string {}
    values["cookies"] = []string {}
    for i := 4; i <= 10; i++ {
	values["file"] = append(values["file"], fmt.Sprintf(testFileName, i))
	values["keys"] = append(values["keys"], strconv.Itoa(i))
	values["cookies"] = append(values["cookies"], strconv.Itoa(i))
    }
    if phase, err = httpPostMultipartAndResp(apiUriUploads, values);
            err != nil {
	t.Errorf("%s: %s error(%v)", "/uploads", phase, err)
	t.FailNow()
    }
    values = nil

    // request api /get
    for i := 1; i <= 10; i++ {
	buf.Reset()
	uri = fmt.Sprintf(apiUriGet, vid, i, i)
	if phase, err = httpGetAndResp(uri, buf); err != nil {
	    t.Errorf("%s %s error(%v)", "/get", phase, err)
	    t.FailNow()
	}
	if err = ioutil.WriteFile(fmt.Sprintf(dlFileName, i), buf.Bytes(),
	        0644); err != nil {
	    t.Errorf("%s error(%v)", "Write File", err)
	    t.FailNow()
	} else {
	    var (
		srcMd5 string
		dlMd5  string
	    )
	    if srcMd5, phase, err = GetMd5OfFile(fmt.Sprintf(testFileName, i));
	            err != nil {
		t.Errorf("%s %s error(%v)", "Calculate MD5", phase, err)
		t.FailNow()
	    }
	    if dlMd5, phase, err = GetMd5OfFile(fmt.Sprintf(dlFileName, i));
	            err != nil {
		t.Errorf("%s %s error(%v)", "Calculate MD5", phase, err)
		t.FailNow()
	    }
	    if srcMd5 != dlMd5 {
		t.Errorf("%s %s", "/get", "MD5 wrong")
		t.FailNow()
	    }
	}
    }

    // request api /del
    for i := 1; i <= 10; i ++ {
	buf.Reset()
	buf.WriteString(fmt.Sprintf("vid=%d&key=%d", vid, i))
	if phase, err = httpPostAndResp(apiUriDel,
	        "application/x-www-form-urlencoded", buf); err != nil {
	    t.Errorf("%s %s error(%v)", "/del", phase, err)
	    t.FailNow()
	}
    }

    // request admin /compact_volume
    buf.Reset()
    buf.WriteString(fmt.Sprintf("vid=%d", vid))
    if phase, err = httpPostAndResp(adminUriCompactVolume,
            "application/x-www-form-urlencoded", buf); err != nil {
	t.Errorf("%s %s error(%v)", "/compact_volume", phase, err)
	t.FailNow()
    }

    time.Sleep(5 * time.Second)
}
