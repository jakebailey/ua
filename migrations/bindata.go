// Code generated by go-bindata.
// sources:
// 1503788894_initial_schema.down.sql
// 1503788894_initial_schema.up.sql
// DO NOT EDIT!

package migrations

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var __1503788894_initial_schemaDownSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x72\x72\x75\xf7\xf4\xb3\xe6\xe2\x72\x09\xf2\x0f\x50\x08\x71\x74\xf2\x71\x55\xc8\xcc\x2b\x2e\x49\xcc\x4b\x4e\x2d\x46\x15\x2e\x2e\x48\x4d\x06\x09\x39\xfb\xfb\xfa\x7a\x86\x58\x73\x01\x02\x00\x00\xff\xff\x91\xfd\x93\x23\x3a\x00\x00\x00")

func _1503788894_initial_schemaDownSqlBytes() ([]byte, error) {
	return bindataRead(
		__1503788894_initial_schemaDownSql,
		"1503788894_initial_schema.down.sql",
	)
}

func _1503788894_initial_schemaDownSql() (*asset, error) {
	bytes, err := _1503788894_initial_schemaDownSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "1503788894_initial_schema.down.sql", size: 58, mode: os.FileMode(493), modTime: time.Unix(1503875223, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var __1503788894_initial_schemaUpSql = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xbc\x90\x41\x4b\xc3\x40\x10\x85\xcf\xd9\x5f\x31\xc7\x16\xf2\x0f\x72\x4a\xcb\x2a\xc1\x24\x95\x35\x1e\x7a\x0a\xd3\xdd\xa1\x8c\xb8\x93\x90\x9d\x48\xf1\xd7\x4b\x50\x8b\x46\xc1\x9b\xd7\xf7\x1e\xf3\xde\x7c\x3b\x7b\x5b\xb5\x85\x31\x7b\x67\xcb\xce\x42\x57\xee\x6a\x0b\x69\x24\x9f\x60\x63\x32\x0e\x30\xcf\x1c\xa0\x3d\x74\xd0\x3e\xd6\x35\xdc\xbb\xaa\x29\xdd\x11\xee\xec\x31\x37\x99\x9f\x08\x95\x42\x8f\x0a\xca\x91\x92\x62\x1c\xf5\xf5\x9a\xce\x4d\x36\x8f\xe1\x8f\x04\xa6\xc4\x67\x89\x24\xda\x0b\x46\x02\xa5\x8b\x7e\xf5\x03\x2a\xc2\x53\x1a\xe4\x74\x55\xcd\xb6\x30\xab\xc5\x2c\x49\x51\x3c\xfd\xd7\xea\x85\x50\xff\xd9\xe3\xec\x8d\x75\xb6\xdd\xdb\x87\x77\x72\x1b\x0e\xdb\xdc\x64\x1c\xf1\x4c\x4b\x68\xfd\x92\x1f\x44\x91\x85\xa6\xdf\x4c\xba\x8c\x3c\x51\x5a\x75\x2f\xa0\xbc\xf2\x0b\xc1\x69\x18\x9e\x09\xe5\xdb\xbd\x45\xa0\xf0\xc3\xfa\xe0\x74\x68\x9a\xaa\x2b\xcc\x5b\x00\x00\x00\xff\xff\x65\x0f\x35\xb8\xea\x01\x00\x00")

func _1503788894_initial_schemaUpSqlBytes() ([]byte, error) {
	return bindataRead(
		__1503788894_initial_schemaUpSql,
		"1503788894_initial_schema.up.sql",
	)
}

func _1503788894_initial_schemaUpSql() (*asset, error) {
	bytes, err := _1503788894_initial_schemaUpSqlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "1503788894_initial_schema.up.sql", size: 490, mode: os.FileMode(493), modTime: time.Unix(1503875223, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"1503788894_initial_schema.down.sql": _1503788894_initial_schemaDownSql,
	"1503788894_initial_schema.up.sql": _1503788894_initial_schemaUpSql,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}
var _bintree = &bintree{nil, map[string]*bintree{
	"1503788894_initial_schema.down.sql": &bintree{_1503788894_initial_schemaDownSql, map[string]*bintree{}},
	"1503788894_initial_schema.up.sql": &bintree{_1503788894_initial_schemaUpSql, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}

