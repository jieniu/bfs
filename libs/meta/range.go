package meta

import (
	"fmt"
	log "github.com/golang/glog"
	"net/http"
	"strconv"
	"strings"
	"xfs/libs/errors"
)

type Range struct {
	Start int64
	End   int64
}

// if file is a bigfile, we should change the file range to the block range
func (r *Range) ConvertRange(maxfilesize int64) (str_range string, err error) {
	var (
		block_index int64
	)
	if r.End < r.Start {
		r.End = 0
	}
	block_index = r.Start / maxfilesize
	if r.End == 0 {
		r.End = maxfilesize - 1
	} else {
		r.End = r.End - maxfilesize*block_index
	}
	if r.End > maxfilesize-1 {
		r.End = maxfilesize - 1
	}
	r.Start = r.Start - maxfilesize*block_index
	str_range = fmt.Sprintf("bytes=%d-%d", r.Start, r.End)
	return
}

func (r *Range) GetRange(str_range string) (err error, status int) {
	r.Start = 0
	r.End = 0
	if str_range == "" {
		return nil, http.StatusOK
	} else {
		var seps_1 []string
		var seps_2 []string
		seps_1 = strings.Split(str_range, "=")
		if len(seps_1) != 2 || seps_1[0] != "bytes" {
			err = errors.ErrParam
			log.Errorf("invalid range param, %s", seps_1)
			return err, http.StatusBadRequest
		}
		seps_2 = strings.Split(seps_1[1], "-")
		if len(seps_2) != 2 {
			err = errors.ErrParam
			log.Errorf("invalid range param, %s", seps_2)
			return err, http.StatusBadRequest
		}
		if r.Start, err = strconv.ParseInt(seps_2[0], 10, 64); err != nil {
			err = errors.ErrParam
			log.Errorf("invalid range param, %s", seps_2)
			return err, http.StatusBadRequest
		}
		if seps_2[1] != "" {
			if r.End, err = strconv.ParseInt(seps_2[1], 10, 64); err != nil {
				err = errors.ErrParam
				log.Errorf("invalid range param, %s", seps_2)
				return err, http.StatusBadRequest
			}
		}
	}
	if r.End != 0 && r.Start > r.End {
		log.Errorf("invalid range param, Start[%d], End[%d]", r.Start, r.End)
		err = errors.ErrParam
		return err, http.StatusBadRequest
	}

	return nil, http.StatusOK
}

func (r *Range) GetSize() (size int64) {
	return r.End - r.Start + 1
}
