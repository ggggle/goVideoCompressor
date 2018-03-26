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
    //"flag"
    "gopkg.in/alecthomas/kingpin.v2"
)
//默认端口
var DEFAULT_PORT string = "8055"
//默认片段大小，单位MB
var DEFAULT_SEGMENT_SIZE = "10"
//touch默认等待时长，单位s
var DEFAULT_TOUCH_TIME = "11"
var (
    myApp      = kingpin.New("server", "Video-convert server.")
    s          = myApp.Command("s", "Split video.")
    sFile      = s.Arg("fileName", "video file name.").Required().String()
    sPieceSize = s.Flag("size", "One piece size.").Default(DEFAULT_SEGMENT_SIZE).Short('s').Uint()
    c          = myApp.Command("c", "Convert video.")
    cFile      = c.Arg("fileName", "video file name.").Required().String()
    cArgs      = c.Flag("ff", "ffmpeg args.").Default("").Short('f').String()
    cPieces    = c.Flag("piece", "只转换一部分片段，用分号;隔开").Short('p').String()
    cPort      = c.Flag("port", "监听端口").Default(DEFAULT_PORT).String()
    t          = myApp.Command("t", "Touch client")
    tTime      = t.Arg("duration", "Duration time").Default(DEFAULT_TOUCH_TIME).Uint()
    tPort      = t.Flag("port", "监听端口").Default(DEFAULT_PORT).Short('p').String()
)

var ConnectId int
var AllConnects = make(map[int]net.Conn)
var OnConnect chan net.Conn
var remainMap = make(map[int]string)
//用来查找remainMap里的key
var initMap = make(map[int]int)

func main() {
    switch kingpin.MustParse(myApp.Parse(os.Args[1:])) {
    case s.FullCommand():
        fmt.Printf("s:[%s] [%d]\n", *sFile, *sPieceSize)
        fileInfo, err := os.Stat(*sFile)
        if err != nil {
            fmt.Printf("[%s]not a file\n", *sFile)
            return
        }
        fileSize := fileInfo.Size()
        //单位转换成MB
        fileSize /= 1024 * 1024
        secondTime := GetSumTime(*sFile)
        segmentTime := int(*sPieceSize) * secondTime / int(fileSize)
        fmt.Printf("segment[%d]s\n", segmentTime)
        segmentArgs := strconv.Itoa(segmentTime)
        fmt.Printf("split video....wait\n")
        b, path := SplitFile(*sFile, segmentArgs)
        fmt.Printf("[%s] [%d]\n", path, b)

    case c.FullCommand():
        fmt.Printf("c:[%s] [%s] [%s]\n", *cFile, *cArgs, *cPieces)
        if "265" == *cArgs{
            *cArgs = "-threads 4 -vcodec libx265 -crf 26"
        } else if "264" == *cArgs{
            *cArgs = "-threads 4 -vcodec libx264"
        }
        pieceNum := fileCount(*cFile)
        if len(*cPieces) > 0 {
            pNum := strings.Split(*cPieces, ";")
            for _, value := range pNum {
                v, err := strconv.Atoi(value)
                if err != nil {
                    fmt.Printf("输入参数错误[%s]\n", *cPieces)
                    return
                } else {
                    //两个映射关系
                    remainMap[v] = value
                    initMap[len(initMap)] = v
                }
            }
        }
        /*现阶段暂时只会填这个参数，忘填-s导致客户端崩溃好几次了 - -
        if len(*cArgs) > 0 {
            *cArgs = "-s 1280x720"
        }*/
        l, err := net.Listen("tcp", "0.0.0.0:" + *cPort)
        defer l.Close()
        if err != nil {
            fmt.Printf("Failure to listen: %s\n", err.Error())
            return
        }
        //去除路径中的最后一个斜杠
        fp := *cFile
        if (fp[len(fp)-1:]) == "/" {
            fp = fp[:len(fp)-1]
        }
        //print(fp + "\n")
        ftpDir := "/home/vuser/" + fp
        os.Mkdir(ftpDir, 0755)
        os.Chown(ftpDir, 9001, 9001)
        //只转换部分片段时
        if len(remainMap) > 0 {
            pieceNum = len(remainMap)
            fmt.Printf("pieceNum[%d]\n", pieceNum)
        } else {
            makeFileList(pieceNum, ftpDir)
            makeConcatScript(ftpDir)
        }
        //go ReadStatus()
        fmt.Printf("pieceNum[%d]\n", pieceNum)
        go JobAlloc(fp, pieceNum, *cArgs)
        for {
            if c, err := l.Accept(); err == nil {
                go NewConnect(c, fp)
            }
        }

    case t.FullCommand():
        fmt.Printf("t:[%d]\n", *tTime)
        l, err := net.Listen("tcp", "0.0.0.0:" + *tPort)
        if err != nil {
            fmt.Printf("Failure to listen: %s\n", err.Error())
            return
        }
        defer l.Close()
        timeOut := make(chan bool, 1)
        go func(second uint) {
            for ; second > 0; second-- {
                time.Sleep(time.Second)
                fmt.Printf("time[%d]\n", second)
            }
            timeOut <- true
        }(*tTime)
        go func() {
            for {
                if c, err := l.Accept(); err == nil {
                    AllConnects[ConnectId] = c
                    ConnectId++
                }
            }
        }()
        select {
        case <-timeOut:
            fmt.Printf("timeout\n")
        }
        fmt.Printf("####Online Client[%d]####\n", ConnectId)
        for key := 0; key < len(AllConnects); key++ {
            fmt.Printf("[%d]Client IP:port[%s]\n", key, AllConnects[key].RemoteAddr().String())
            AllConnects[key].Close()
        }
        fmt.Printf("time[%s]\n", time.Now().Format("2006-01-02 15:04:05"))
    }
    return
}

func JobAlloc(path string, num int, convertArgs string) {
    //fmt.Printf("%v\n", remainMap)
    //remainMap在外构造，只转换部分片段
    if len(remainMap) == 0 {
        for i := 0; i < num; i++ {
            remainMap[i] = strconv.Itoa(i)
            initMap[i] = i
        }
    }
    num--
    fmt.Printf("num [%d]\n", num)
    fmt.Printf("remainMap %v\n", remainMap)
    fmt.Printf("initMap %v\n", initMap)
    OnConnect = make(chan net.Conn, 50)
Loop:
    for {
        select {
        case i := <-OnConnect:
            _, err := i.Write([]byte(path + ";" + remainMap[initMap[num]] + ";" + convertArgs))
            if err != nil {
                fmt.Printf("!!!send error[%s]\n", err.Error())
            } else {
                fmt.Printf("[%s]OnConnect send success[%s]\n", time.Now().Format("2006-01-02 15:04:05"), path+";"+remainMap[initMap[num]])
                num--
            }
            if num < 0 {
                break Loop
            }
        }
    }
}

//分割文件
func SplitFile(filePath string, segment_time string) (piece int, dir string) {
    //替换\为. 创建目录，分割出来的片段存到目录下
    dir = strings.Replace(filePath, "\\", ".", -1)
    dir = "12" + dir
    os.Mkdir(dir, 0755)
    cmd := exec.Command("ffmpeg", "-i", filePath, "-acodec", "copy", "-f", "segment", "-segment_time",
        segment_time, "-vcodec", "copy", "-reset_timestamps", "1", "-map", "0:0", "-map", "0:1", dir+"/"+"%d.mp4")
    cmd.Run()
    piece = fileCount(dir)
    return
}

//返回文件数量，即最大序号+1
func fileCount(path string) (fileNum int) {
    err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
        if info == nil {
            return err
        }
        if info.IsDir() {
            return nil
        }
        //fmt.Println(path)
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

//返回视频秒数
func GetSumTime(filePath string) (SumTime int) {
    cmd := exec.Command("ffmpeg", "-i", filePath)
    w := bytes.NewBuffer(nil)
    cmd.Stderr = w
    cmd.Run()
    //fmt.Printf("stderr: %s\n", string(w.Bytes()))
    //匹配出时间长度
    reg := regexp.MustCompile("Duration:(\\s+)(\\d+):(\\d+):(\\d+)")
    timeStr := []rune(reg.FindString(string(w.Bytes())))[len("Duration: "):]
    //计算秒数
    for key, value := range strings.Split(string(timeStr), ":") {
        i, _ := strconv.Atoi(value)
        switch key {
        case 0:
            SumTime += i * 3600
        case 1:
            SumTime += i * 60
        case 2:
            SumTime += i
        }
        //fmt.Printf("%s\n", value)
    }
    SumTime ++
    fmt.Printf("视频长度[%d]s\n", SumTime)
    return
}

func NewConnect(c net.Conn, dirPath string) {
    fmt.Printf("new connect\n")
    go func() {
        for {
            data := make([]byte, 100)
            n, err := c.Read(data)
            if err == nil {
                Data := string(data[:n])
                switch {
                case Data[0:4] == "fail":
                    retSplit := strings.Split(Data, ";")
                    fmt.Printf("@@[%s]piece convert fail\n", retSplit[1])
                    if len(retSplit) >= 3{
                        fmt.Printf("@@reason[%s]\n", retSplit[2])
                    }
                    c.Close()
                    return
                case Data[0:7] == "success":
                    pieceNum, _ := strconv.Atoi(strings.Split(Data, ";")[1])
                    fmt.Printf("###[%s] [%d]piece convert success\n", time.Now().Format("2006-01-02 15:04:05"), pieceNum)
                    c.Close()
                    os.Remove(dirPath + "/" + strings.Split(Data, ";")[1] + ".mp4")
                    delete(remainMap, pieceNum)
                    if remainJob := len(remainMap); remainJob == 0 {
                        fmt.Printf("------Convert All Done-----\n")
                        os.Exit(0)
                    } else {
                        str := make([]string, 0)
                        for _, value := range remainMap {
                            str = append(str, value)
                        }
                        fmt.Printf("[%d]remain job[%v]\n", remainJob, strings.Join(str, ";"))
                    }
                    return
                default:
                    continue
                }
            }
        }
    }()
    OnConnect <- c
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

func makeFileList(fileCount int, dirPath string) {
    var filePath string
    fileName := "filelist.txt"
    if dirPath[len(dirPath)-1:] == "/" {
        filePath = dirPath + fileName
    } else {
        filePath = dirPath + "/" + fileName
    }
    _, err := os.Stat(filePath)
    //文件不存在
    if err != nil && os.IsNotExist(err) {
        file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
        if err != nil {
            fmt.Printf("makeFileList error[%v]\n", err)
            return
        }
        for i := 0; i < fileCount; i++ {
            content := fmt.Sprintf("file '%d.mp4'\n", i)
            file.WriteString(content)
        }
        file.Close()
    }
}

func makeConcatScript(dirPath string) {
    file := dirPath + "/" + "concat.sh"
    _, err := os.Stat(file)
    if err != nil && os.IsNotExist(err) {
        f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0755)
        if err != nil {
            fmt.Printf("makeConcatScript error[%v]\n", err)
        } else {
            f.WriteString("#!/bin/sh\nffmpeg -f concat -i filelist.txt -c copy output.mp4")
            f.Close()
        }
    }
}
