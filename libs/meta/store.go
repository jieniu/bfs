package meta

import (
	"encoding/json"
	"fmt"
	log "github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"xfs/libs/errors"
)

const (
	// bit
	StoreStatusEnableBit = 31
	StoreStatusReadBit   = 0
	StoreStatusWriteBit  = 1
	// status
	StoreStatusInit   = 0
	StoreStatusEnable = (1 << StoreStatusEnableBit)
	StoreStatusRead   = StoreStatusEnable | (1 << StoreStatusReadBit)
	StoreStatusWrite  = StoreStatusEnable | (1 << StoreStatusWriteBit)
	StoreStatusHealth = StoreStatusRead | StoreStatusWrite
	StoreStatusFail   = StoreStatusEnable
	// api
	statAPI  = "http://%s/info"
	getAPI   = "http://%s/get?key=%d&cookie=%d&vid=%d"
	probeAPI = "http://%s/probe?vid=%d"
)

type StoreList []*Store

func (sl StoreList) Len() int {
	return len(sl)
}

func (sl StoreList) Less(i, j int) bool {
	return sl[i].Id < sl[j].Id
}

func (sl StoreList) Swap(i, j int) {
	sl[i], sl[j] = sl[j], sl[i]
}

// store zk meta data.
type Store struct {
	Stat   string `json:"stat"`
	Admin  string `json:"admin"`
	Api    string `json:"api"`
	Id     string `json:"id"`
	Rack   string `json:"rack"`
	Status int    `json:"status"`
}

func (s *Store) String() string {
	return fmt.Sprintf(`	
-----------------------------
Id:     %s
Stat:   %s
Admin:  %s
Api:    %s
Rack:   %s
Status: %d
-----------------------------
`, s.Id, s.Stat, s.Admin, s.Api, s.Rack, s.Status)
}

// statAPI get stat http api.
func (s *Store) statAPI() string {
	return fmt.Sprintf(statAPI, s.Stat)
}

// getApi get file http api
func (s *Store) getAPI(n *Needle, vid int32) string {
	return fmt.Sprintf(getAPI, s.Stat, n.Key, n.Cookie, vid)
}

// probeApi probe store
func (s *Store) probeAPI(vid int32) string {
	return fmt.Sprintf(probeAPI, s.Admin, vid)
}

// Info get store volumes info.
func (s *Store) Info() (vs []*Volume, err error) {
	var (
		body []byte
		resp *http.Response
		data = new(Volumes)
		url  = s.statAPI()
	)
	if resp, err = http.Get(url); err != nil {
		log.Warningf("http.Get(\"%s\") error(%v)", url, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.ErrInternal
		return
	}
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		log.Errorf("ioutil.ReadAll() error(%v)", err)
		return
	}
	if err = json.Unmarshal(body, &data); err != nil {
		log.Errorf("json.Unmarshal() error(%v)", err)
		return
	}
	vs = data.Volumes
	return
}

// Head send a head request to store.
func (s *Store) Head(vid int32) (err error) {
	var (
		resp *http.Response
		url  string
	)
	url = s.probeAPI(vid)
	if resp, err = http.Head(url); err != nil {
		return
	}
	if resp.StatusCode == http.StatusInternalServerError {
		err = errors.ErrInternal
	}
	return
}

// CanWrite reports whether the store can write.
func (s *Store) CanWrite() bool {
	return s.Status == StoreStatusWrite || s.Status == StoreStatusHealth
}

// CanRead reports whether the store can read.
func (s *Store) CanRead() bool {
	return s.Status == StoreStatusRead || s.Status == StoreStatusHealth
}
