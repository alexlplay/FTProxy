package parseindex

import "fmt"
import "golang.org/x/net/html"
import "cfg"
import "net/http"
import "strings"
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
    size int64
}

func GenDirList(objects []FsObject) (string) {
    var listing string
    for _, object := range objects {
        if object.otype == FS_DIR {
            listing = fmt.Sprintf("%sdrwxr-xr-x 1 0 0 1 Dec 13 10:40 %s\r\n", listing, object.name)
        } else if object.otype == FS_FILE {
            listing = fmt.Sprintf("%s-rwxr-xr-x 1 0 0 %d Dec 13 10:40 %s\r\n", listing, object.size, object.name)
        }
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

type Parser interface {
    Parse(path string) (string, bool)
}

type ParserAutoIndex struct {
}

func (p ParserAutoIndex) Parse(path string) (string, bool) {
    cfg.LoadConfig("ftproxy.conf")
    vhost := cfg.GetVhost(path)
    url := fmt.Sprintf("http://%s/%s/", vhost, path)
    fmt.Printf("URL FOR INDEX: %s\n", url)
    resp, err := http.Get(url)
    if err != nil {
        return "", false
    }
    defer resp.Body.Close()
    var objects []FsObject
    fmt.Printf("Server header: %s\n", resp.Header["Server"][0])
    if strings.Contains(resp.Header["Server"][0], "nginx") {
        objects = ParseNginxHtmlList(resp.Body)
    } else {
        // Attempt apache
        objects = ParseApacheHtmlList(resp.Body)
    }
    return GenDirList(objects), true
}

type ParserConf struct {
}

func (p ParserConf) Parse(truc string) (string, bool) {
    cfg.LoadConfig("ftproxy.conf")
    vhosts := cfg.GetVhosts()
    var listing string
    for path, _  := range vhosts {
        listing = fmt.Sprintf("%sdrwxr-xr-x 1 0 0 1 Dec 13 10:40 %s\r\n", listing, path)
    }
    return listing, true
      //      listing = fmt.Sprintf("%sdrwxr-xr-x 1 0 0 1 Dec 13 10:40 %s\r\n", listing, object.name)
}
