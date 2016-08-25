package util

import (
	log "github.com/golang/glog"
	"strings"
	"xfs/libs/errors"
)

func GetParentDir(dir string) (parent string, err error) {
	if len(dir) > 1 {
		parent_pos := strings.LastIndex(dir[:len(dir)-1], "/")
		if parent_pos >= 0 {
			parent = dir[:parent_pos+1]
		} else {
			log.Warningf("it's a root dir or please check your parameter: dir[%s]", dir)
			return parent, nil
		}
	} else {
		log.Warningf("it's a root dir or please check your parameter: dir[%s]", dir)
		err = errors.ErrInternal
		parent = ""
	}
	return parent, nil
}
