package main

import (
    "net"
    "fmt"
    "time"
    "os"
    "strings"
    "os/exec"
    "bytes"
    "regexp"
    "strconv"
    "path/filepath"
)

var ConnectId int
var AllConnects = make(map[int]net.Conn)
var TickTime int = 10
var OnConnect chan int
var ConvertSuccess chan [2]int

func main() {
    //b,path:=SplitFile("supa-159.mp4")
    //fmt.Printf("%d", b)
    l, err := net.Listen("tcp", "0.0.0.0:8054")
    defer l.Close()
    if err != nil {
        fmt.Printf("Failure to listen: %s\n", err.Error())
    }
    //go HeartBeat()
    go ReadStatus()
    go JobAlloc("supa-159.mp412", 20, "-s 1280x720")
    for {
        if c, err := l.Accept(); err == nil {
            go NewConnect(c, ConnectId)
            ConnectId++
        }
    }
    fmt.Printf("%d\n", fileCount("./"))
}

func JobAlloc(path string, num int, convertArgs string) {
    remainMap := make(map[int]string)
    for i := 0; i < num; i++ {
        remainMap[i] = strconv.Itoa(i)
    }
    for _, c := range AllConnects {
        fmt.Printf("job[%d]to[%s]", num, c.RemoteAddr().String())
        numStr := strconv.Itoa(num)
        _, err := c.Write([]byte(path + ";" + numStr + ";" + convertArgs))
        if err != nil {
            fmt.Printf("send error[%s]", err.Error())
        } else {
            num--
            fmt.Printf("first alloc[%s]\n", path+";"+numStr)
        }
    }
    ConvertSuccess = make(chan [2]int, 10)
    OnConnect = make(chan int, 10)
Loop:
    for {
        select {
        case i := <-ConvertSuccess:
            AllConnects[i[0]].Close()
            delete(AllConnects, i[0])
            delete(remainMap, i[1])
            fmt.Printf("piece[%d] convert success\n", i[1])
            /*
            numStr := strconv.Itoa(num)
            _, err := AllConnects[i[0]].Write([]byte(path + ";" + numStr + ";" + convertArgs))
            if err != nil {
                fmt.Printf("!!!send error[%s]\n", err.Error())
            } else {
                num--
                fmt.Printf("ConvertSuccess send success[%d] [%s]\n", i, path+";"+numStr)
            }
            if num < 0 {
                break Loop
            }
            */
        case i := <-OnConnect:
            numStr := strconv.Itoa(num)
            _, err := AllConnects[i].Write([]byte(path + ";" + numStr + ";" + convertArgs))
            if err != nil {
                fmt.Printf("!!!send error[%s]\n", err.Error())
            } else {
                num--
                fmt.Printf("OnConnect send success[%d] [%s]\n", i, path+";"+numStr)
            }
            if num < 0 {
                break Loop
            }
        }
    }
}

func ReadStatus() {
    for {
        for key, c := range AllConnects {
            data := make([]byte, 100)
            n, err := c.Read(data)
            if err == nil {
                Data := string(data[:n])
                switch {
                case Data[0:7] == "success":
                    var arr [2]int
                    arr[0] = key
                    pieceNum := strings.Split(Data, ";")[1]
                    arr[1], _ = strconv.Atoi(pieceNum)
                    fmt.Printf("[%d] convert success\n", key)
                    ConvertSuccess <- arr
                default:
                    fmt.Printf("heart[%d]\n", key)
                }
            }
        }
        time.Sleep(time.Second * 10)
    }
}

//分割文件
func SplitFile(filePath string) (piece int, dir string) {
    //替换\为. 创建目录，分割出来的片段存到目录下
    dir = strings.Replace(filePath, "\\", ".", -1)
    print(dir)
    dir += "12"
    os.Mkdir(dir, 0755)
    /*timeSum := GetSumTime(filePath)
    //w := bytes.NewBuffer(nil)
    //TickTime 每块视频的大小
    for start:=0; start<timeSum; piece, start = piece+1, start+TickTime {
        fmt.Printf("start[%d] piece[%d]\n", start, piece)
        pieceName := strconv.Itoa(piece) + ".mp4"
        cmd := exec.Command("ffmpeg", "-i", filePath, "-ss", FormatTime(start),
            "-t", FormatTime(TickTime), "-codec", "copy", dir+"/"+pieceName)
        //cmd.Stderr = w
        cmd.Run()
        //fmt.Printf("%s\n", string(w.Bytes()))
    }*/
    cmd := exec.Command("ffmpeg", "-i", filePath, "-acodec", "copy", "-f", "segment", "-vcodec",
        "copy", "-reset_timestamps", "1", "-map", "0", dir+"/"+"%d.mp4")
    cmd.Run()
    piece = fileCount(dir)
    return
}

func fileCount(path string) (fileNum int) {
    err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
        if info == nil {
            return err
        }
        if info.IsDir() {
            return nil
        }
        fmt.Println(path)
        fileNum++
        return nil
    })
    if err != nil {
        fmt.Printf("error [%v]\n", err)
    }
    return
}

func FormatTime(minute int) (formatString string) {
    strSlice := make([]string, 3)
    hh := minute / 60
    mm := minute % 60
    strSlice[0] = "0" + strconv.Itoa(hh)
    strSlice[1] = strconv.Itoa(mm)
    strSlice[2] = "00"
    if len(strSlice[1]) == 1 {
        strSlice[1] = "0" + strSlice[1]
    }
    formatString = strings.Join(strSlice, ":")
    return
}

//返回视频分钟数
func GetSumTime(filePath string) (SumTime int) {
    cmd := exec.Command("ffmpeg", "-i", filePath)
    w := bytes.NewBuffer(nil)
    cmd.Stderr = w
    cmd.Run()
    //fmt.Printf("stderr: %s\n", string(w.Bytes()))
    //匹配出时间长度
    reg := regexp.MustCompile("Duration:(\\s+)(\\d+):(\\d+):(\\d+)")
    timeStr := []rune(reg.FindString(string(w.Bytes())))[len("Duration: "):]
    //计算分钟数
    for key, value := range strings.Split(string(timeStr), ":") {
        i, _ := strconv.Atoi(value)
        switch key {
        case 0:
            SumTime += i * 60
        case 1:
            SumTime += i
        case 2:
            if i > 0 {
                SumTime ++
            }
        }
        fmt.Printf("%s\n", value)
    }
    fmt.Printf("%d\n", SumTime)
    return
}

func NewConnect(c net.Conn, i int) {
    fmt.Printf("new connect\n")
    AllConnects[i] = c
    OnConnect <- i
}

func HeartBeat() {
    for {
        for key, c := range AllConnects {
            fmt.Printf("send heartbest to[%d]\n", key)
            _, err := c.Write([]byte("heart"))
            if err != nil {
                fmt.Printf("send heart error[%s] delete[%d]\n", err.Error(), key)
                delete(AllConnects, key)
            }
        }
        time.Sleep(time.Second * 3)
    }

}
