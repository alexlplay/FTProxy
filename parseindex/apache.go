package parseindex

import "fmt"
import "golang.org/x/net/html"
import "io"
import "regexp"
import "strconv"
import "strings"


func ParseApacheHtmlList(r io.Reader) ([]FsObject) {
    z := html.NewTokenizer(r)

    var inFirstTable bool
    var inTr bool
    var inTd bool
    var tdNumber int // td number within current tr
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

        if t.Data == "table" {
            inFirstTable = true
        }
        if inFirstTable == true && t.Data == "tr" {
            inTr = true
            tdNumber = 0
            curObj.otype = FS_NONE
            curObj.name = ""
            curObj.size = 0
        }
        if inTr == true && t.Data == "td" {
            inTd = true
            tdNumber++
            if tdNumber == 4 {  // Size
                z.Next()
                sizeText := z.Token()
                suffixMult := map[string]float64{
                    "K": 1024,
                    "M": 1024*1024,
                    "G": 1024*1024*1024,
                    "T": 1024*1024*1024*1024,
                }
                fmt.Printf(" size: %s ", sizeText)
                re := regexp.MustCompile("([0-9]+(?:\\.?[0-9]+)?)([K|M|])?")
                match := re.FindStringSubmatch(sizeText.String())
                if match == nil {
                    fmt.Println("No size regex match")
                } else {
                    preSize, err := strconv.ParseFloat(match[1], 64)
                    if err == nil {
                        if match[2] != "" {
                            curObj.size = int64(preSize * suffixMult[match[2]])
                        } else {
                            curObj.size = int64(preSize)
                        }
                        fmt.Printf("match: %d\n", curObj.size)
                    } else {
                        // Better lie about the size than return empty which will confuse the client even more
                        fmt.Println(err)
                        curObj.size = 3
                    }
                } 
            }
        }
        if inTd == true && t.Data == "a" {
            inTd = true
            fmt.Printf("link: %s ", getTokenAttr(&t, "href"))
            curObj.name = getTokenAttr(&t, "href")
        }
        if inTd == true && t.Data == "img" {
            imgText := getTokenAttr(&t, "alt")
            //fmt.Printf("alt: %s ", imgText)
            if strings.Contains(imgText, "DIR") {
                curObj.otype = FS_DIR
            } else if strings.Contains(imgText, "[ ") {
                curObj.otype = FS_FILE
            }
        }

    case html.EndTagToken:
        t := z.Token()
        if t.Data == "table" {
            // return objects 
            inFirstTable = false // ugly, scan all tables
        }
        if t.Data == "tr" {
            inTr = false
            if curObj.name != "" && curObj.otype != FS_NONE {
                // fmt.Println(curObj)
                objects = append(objects, *curObj)
            }
        }
        if t.Data == "td" {
            inTd = false
        }
    }
}
}
