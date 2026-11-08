package ccfs

import (
	"errors"
	"io/fs"
)

// ErrInCloud refuses to read a file whose content lies in the cloud, where
// a thumbnail, a checksum or a text search would fetch every byte of it.
var ErrInCloud = errors.New("the file lies in the cloud only")

// The attributes a cloud provider such as OneDrive sets on a placeholder,
// with the values of winnt.h.
const (
	attrOffline            = 0x1000
	attrRecallOnOpen       = 0x40000
	attrRecallOnDataAccess = 0x400000
)

// CloudOnly reports whether reading a file with these attributes of
// Windows would download it first.
func CloudOnly(attrs uint32) bool {
	return attrs&(attrOffline|attrRecallOnOpen|attrRecallOnDataAccess) != 0
}

// InCloudInfo reports whether info describes such a file. The attributes
// came along with info, so it costs nothing.
func InCloudInfo(info fs.FileInfo) bool { return CloudOnly(infoAttrs(info)) }
