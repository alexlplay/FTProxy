package parseindex

import "fmt"
import "golang.org/x/net/html"
import "io"
import "regexp"
import "strconv"
import "strings"
import "time"

var prenom string

func ParseNginxHtmlList(r io.Reader) ([]FsObject) {
    //allFile, _ := ioutil.ReadAll(r)
    //fmt.Printf("%s\n", allFile)
    z := html.NewTokenizer(r)

    curObj := new(FsObject)
    var objects []FsObject

    for {
    tt := z.Next()

    switch tt {
    case html.ErrorToken:
        // End of the document, we're done
        return objects
    case html.StartTagToken:
        t := z.Token()

        if t.Data == "a" {
            fmt.Printf("link: %s ", getTokenAttr(&t, "href"))
            curObj.name = getTokenAttr(&t, "href")
            curObj.name = strings.Trim(curObj.name, "/")
            curObj.otype = FS_NONE
            curObj.size = 0
        }
    case html.EndTagToken:
        t := z.Token()
        if t.Data == "a" {
            // return objects 
            fmt.Println("end link")
            z.Next()
            dateAndSizeText := z.Token()
            re := regexp.MustCompile("([0-9][0-9]-...-[0-9]+ [0-9][0-9]:[0-9][0-9])\\s+([0-9]+|-)")
            match := re.FindStringSubmatch(dateAndSizeText.String())
            if match == nil {
                fmt.Println("No date and size regex match")
            } else {
                // match[1] should contain date and time, match[2] size in bytes
                tim, err := time.Parse("_2-Jan-2006 15:04", match[1])
                if err == nil {
                    curObj.time = tim
                } else {
                    // Failed to parse time
                    curObj.time = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
                }
                if match[2] == "-" {
                    curObj.size = 4096 /* XXX fake size */
                    curObj.otype = FS_DIR
                } else {
                    floatSize, err := strconv.ParseFloat((match[2]), 64)
                    if err == nil {
                        curObj.size = int64(floatSize)
                        curObj.otype = FS_FILE
                    }
                }
                if curObj.name != "" && curObj.otype != FS_NONE {
                    objects = append(objects, *curObj)
                }
            }
        }
    }
    }
}
