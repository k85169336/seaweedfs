package weed_server

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/chrislusf/seaweedfs/weed/filer"
	"github.com/chrislusf/seaweedfs/weed/glog"
	"github.com/chrislusf/seaweedfs/weed/operation"
	"github.com/chrislusf/seaweedfs/weed/security"
	ui "github.com/chrislusf/seaweedfs/weed/server/filer_ui"
	"github.com/chrislusf/seaweedfs/weed/util"
	"github.com/syndtr/goleveldb/leveldb"
)

// listDirectoryHandler lists directories and folers under a directory
// files are sorted by name and paginated via "lastFileName" and "limit".
// sub directories are listed on the first page, when "lastFileName"
// is empty.
func (fs *FilerServer) listDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/") {
		return
	}
	limit, limit_err := strconv.Atoi(r.FormValue("limit"))
	if limit_err != nil {
		limit = 100
	}

	lastFileName := r.FormValue("lastFileName")
	files, err := fs.filer.ListFiles(r.URL.Path, lastFileName, limit)

	if err == leveldb.ErrNotFound {
		glog.V(0).Infof("Error %s", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	directories, err2 := fs.filer.ListDirectories(r.URL.Path)
	if err2 == leveldb.ErrNotFound {
		glog.V(0).Infof("Error %s", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	shouldDisplayLoadMore := len(files) > 0

	lastFileName = ""
	if len(files) > 0 {
		lastFileName = files[len(files)-1].Name

		files2, err3 := fs.filer.ListFiles(r.URL.Path, lastFileName, limit)
		if err3 == leveldb.ErrNotFound {
			glog.V(0).Infof("Error %s", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		shouldDisplayLoadMore = len(files2) > 0
	}

	args := struct {
		Path                  string
		Files                 interface{}
		Directories           interface{}
		Limit                 int
		LastFileName          string
		ShouldDisplayLoadMore bool
	}{
		r.URL.Path,
		files,
		directories,
		limit,
		lastFileName,
		shouldDisplayLoadMore,
	}

	if r.Header.Get("Accept") == "application/json" {
		writeJsonQuiet(w, r, http.StatusOK, args)
	} else {
		ui.StatusTpl.Execute(w, args)
	}
}

func (fs *FilerServer) GetOrHeadHandler(w http.ResponseWriter, r *http.Request, isGetMethod bool) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Max-Age", "1000")

	if strings.HasSuffix(r.URL.Path, "/") {
		if fs.disableDirListing {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		fs.listDirectoryHandler(w, r)
		return
	}

	//本地找不到就去fs.syncFile查找
	fileId, err := fs.filer.FindFile(r.URL.Path)
	if err == filer.ErrNotFound {
		if fs.syncFile != "" {
			// data, fileName, contentType, err := util.Download(fs.syncFile + r.URL.Path)
			tmpFile, fileName, contentType, err := util.NewDownload(fs.syncFile + r.URL.Path)
			if err != nil {
				glog.V(0).Infoln(r.URL.Path, err)
				w.WriteHeader(http.StatusNotFound)
				os.Remove(tmpFile)
				return
			}
			data, err := ioutil.ReadFile(tmpFile)
			if err != nil {
				glog.V(0).Infoln(r.URL.Path, err)
				w.WriteHeader(http.StatusNotFound)
				os.Remove(tmpFile)
				return
			}
			jwt := security.GetJwt(r)
			// _, err = operation.Upload("http://"+r.Host+r.URL.Path, fileName, bytes.NewReader(data), false, contentType, nil, jwt)
			_, err = operation.Upload("http://"+r.Host+r.URL.Path, fileName, bytes.NewReader(data), false, contentType, nil, jwt)
			if err != nil {
				glog.V(0).Infoln("upload", err)
				w.WriteHeader(http.StatusNotFound)
				os.Remove(tmpFile)
				return
			} else {
				glog.V(0).Infoln("sync", fs.syncFile+r.URL.Path)
				os.Remove(tmpFile)
			}
		}
	}

	reqUrl := r.URL.Path
	var reqQuery string
	if r.FormValue("w") != "" {
		reqQuery += "&w=" + r.FormValue("w")
	}
	if r.FormValue("h") != "" {
		reqQuery += "&h=" + r.FormValue("h")
	}
	if r.FormValue("r") != "" {
		reqQuery += "&r=" + r.FormValue("r")
	}
	if r.FormValue("w") != "" && r.FormValue("h") != "" && r.FormValue("f") != "" {
		reqQuery += "&f=" + r.FormValue("f")
	}
	if reqQuery = strings.TrimLeft(reqQuery, "&"); reqQuery != "" {
		reqUrl += "?" + reqQuery
	}
	fileId, err = fs.filer.FindFile(reqUrl)
	if err == filer.ErrNotFound {
		glog.V(0).Infoln(reqUrl, "not exist")
		r.Header.Add("exist", "0")
		r.Header.Add("path", r.URL.Path)
		fileId, err = fs.filer.FindFile(r.URL.Path)
		if err == filer.ErrNotFound {
			glog.V(0).Infoln(r.URL.Path, "not exist")
			w.WriteHeader(http.StatusNotFound)
			return
		}
	} else {
		glog.V(0).Infoln(reqUrl, "exist")
		r.Header.Add("exist", "1")
	}

	urlLocation, err := operation.LookupFileId(fs.getMasterNode(), fileId)
	if err != nil {
		glog.V(1).Infoln("operation LookupFileId %s failed, err is %s", fileId, err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	urlString := urlLocation
	if fs.redirectOnRead {
		http.Redirect(w, r, urlString, http.StatusFound)
		return
	}
	u, _ := url.Parse(urlString)
	q := u.Query()
	for key, values := range r.URL.Query() {
		for _, value := range values {
			q.Add(key, value)
		}
	}
	u.RawQuery = q.Encode()
	request := &http.Request{
		Method:        r.Method,
		URL:           u,
		Proto:         r.Proto,
		ProtoMajor:    r.ProtoMajor,
		ProtoMinor:    r.ProtoMinor,
		Header:        r.Header,
		Body:          r.Body,
		Host:          r.Host,
		ContentLength: r.ContentLength,
	}
	glog.V(3).Infoln("retrieving from", u)
	resp, do_err := util.Do(request)
	if do_err != nil {
		glog.V(0).Infoln("failing to connect to volume server", do_err.Error())
		writeJsonError(w, r, http.StatusInternalServerError, do_err)
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

}
