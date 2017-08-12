package weed_server

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/chrislusf/seaweedfs/weed/filer"
	"github.com/chrislusf/seaweedfs/weed/glog"
	"github.com/chrislusf/seaweedfs/weed/operation"
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
	fileId, err := fs.filer.FindFile(reqUrl)
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

func download(u string) (data []byte, fileName, contentType string, err error) {
	client := &http.Client{}
	request, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return
	}

	resp, err := client.Do(request)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = fmt.Errorf("%s", "下载失败")
	}
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	contentType = resp.Header.Get("Content-type")

	ur, err := url.Parse(u)
	if err != nil {
		return
	}
	_, fileName = path.Split(ur.Path)

	return
}
