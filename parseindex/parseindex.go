package parseindex

import "fmt"
import "golang.org/x/net/html"
import "cfg"
import "net/http"
import "path"
import "strings"
import "time"
//import "bufio"

type FsObjectType int
const (
    FS_NONE FsObjectType = iota
    FS_DIR
    FS_FILE
)

type FsObject struct {
    otype FsObjectType
    name string
    time time.Time
    size int64
}

func GenDirList(objects []FsObject) (string) {
    var listing string
    for _, object := range objects {
        var printTime string
        var lineHdr string
        if(object.time.Year() < time.Now().Year()) {
            printTime = object.time.Format("Jan _2  2006")
        } else {
            printTime = object.time.Format(time.Stamp)
        }
        if object.otype == FS_DIR {
            lineHdr = "d"
        } else {
            lineHdr = "-"
        }
        listing = fmt.Sprintf("%s%srwxr-xr-x 1 ftp ftp %d %s %s\r\n", listing, lineHdr, object.size, printTime, object.name)
    }
    return listing
}

func getTokenAttr(tok *html.Token, attrName string) (string) {
    for _, a := range tok.Attr {
        if a.Key == attrName {
            return a.Val
        }
    }
    return ""
}

func GetFSObjects(dirName string) ([]FsObject) {
    cfg.LoadConfig("ftproxy.conf")
    dirName = path.Clean(dirName)
    var objects []FsObject

    if dirName == "/" {
        vhosts := cfg.GetVhosts()
        fmt.Printf("GetFSObjects(): dir: %s, generating fake listing from vhosts\n", dirName)
        curObj := new(FsObject)
        for path, _  := range vhosts {
            // Generate fake timestamps for first-level directories (our list of vhosts)
            curObj.name = strings.Trim(path, "/")
            curObj.time = time.Now()
            curObj.size = 4096 /* XXX fake size */
            curObj.otype = FS_DIR
            objects = append(objects, *curObj)
        }
    } else {
        vhost := cfg.GetVhost(dirName)
        url := fmt.Sprintf("http://%s%s", vhost, dirName)
        fmt.Printf("GetFSObjects(): dir: %s, url: %s\n", dirName, url)
        resp, err := http.Get(url)
        if err != nil {
            return objects
        }
        defer resp.Body.Close()
        fmt.Printf("Server header: %s\n", resp.Header["Server"][0])
        if strings.Contains(resp.Header["Server"][0], "nginx") {
            objects = ParseNginxHtmlList(resp.Body)
        } else {
            objects = ParseApacheHtmlList(resp.Body)
        }
    }
    return objects
}

func DirList(path string) (string, bool) {
    objects := GetFSObjects(path)
    return GenDirList(objects), true
}

func FileStat(filePath string) (int64, string, bool) {
    dirName, fileName := path.Split(filePath)
    objects := GetFSObjects(dirName)
    for _, object := range objects {
        if object.name == fileName && object.otype == FS_FILE {
            fmt.Printf("Found file, size is: %d, time is: %s\n", object.size, object.time)
            return object.size, object.time.Format("20060102030405"), true
        }
    }
    return 0, "", false
}
