package meta

import (
	"bfs/libs/errors"
	log "github.com/golang/glog"
	"net/http"
	"strconv"
	"strings"
)

type Range struct {
	Start int
	End   int
}

func (r *Range) GetRange(str_range string, p_range *Range) (err error, status int) {
	p_range.Start = 0
	p_range.End = 0
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
		if p_range.Start, err = strconv.Atoi(seps_2[0]); err != nil {
			err = errors.ErrParam
			log.Errorf("invalid range param, %s", seps_2)
			return err, http.StatusBadRequest
		}
		if seps_2[1] != "" {
			if p_range.End, err = strconv.Atoi(seps_2[1]); err != nil {
				err = errors.ErrParam
				log.Errorf("invalid range param, %s", seps_2)
				return err, http.StatusBadRequest
			}
		}
	}
	if p_range.End != 0 && p_range.Start > p_range.End {
		log.Errorf("invalid range param, Start[%d], End[%d]", p_range.Start, p_range.End)
		err = errors.ErrParam
		return err, http.StatusBadRequest
	}

	return nil, http.StatusOK
}
