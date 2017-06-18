package main

import (
    "net";
    "fmt"
    "time"
    "os/exec"
    "strings"
    "github.com/dutchcoders/goftp"
    "os"
    "bytes"
)

var printLog bool = true
var converSuccess chan string = make(chan string, 10)
var nginxServer string = "http://188.166.213.154/"

func main() {
    for {
        connect, err := net.DialTimeout("tcp", "188.166.213.154:8054", time.Second*10)
        ipStr := ""
        if err == nil {
            MyPrintf("connect success\n")
            ipStr = connect.LocalAddr().String()
            data := make([]byte, 100)
            MyPrintf("i'm here\n")
            n, err := connect.Read(data)
            if err != nil {
                MyPrintf("error info[%s]\n", err.Error())
                continue
            } else {
                Data := data[:n]
                //Data  dir;%d;args
                if recieveSplit := strings.Split(string(Data), ";"); len(recieveSplit) > 1 {
                    //path   dir/%d.mp4
                    path := strings.Join(recieveSplit[:2], "/") + ".mp4"
                    MyPrintf("args [%v]\n", recieveSplit)
                    go downloadFileAndConvert(path, recieveSplit[2])
                }
            }
        Loop:
            for {
                select {
                case status := <-converSuccess:
                    if len(status) > 0 {
                        MyPrintf("tell server[%s]\n", status)
                        _, err := connect.Write([]byte("success;" + status))
                        if err == nil {
                            MyPrintf("tell server success\n")
                            connect.Close()
                            break Loop
                        }
                    }
                }
            }

        } else {
            MyPrintf("connect error[%s]\n", err.Error())
            time.Sleep(time.Second * 10)
        }
        if connect != nil {
            connect.Close()
            MyPrintf("disconnect[%s]\n", ipStr)
        }
    }
}

func downloadFileAndConvert(path string, args string) {
    cmd := exec.Command("wget", nginxServer+path)
    cmd.Run()
    //path   dir/%d.mp4
    convert(strings.Split(path, "/")[1], args)
    go uploadFile("c"+strings.Split(path, "/")[1], strings.Split(path, "/")[0])
    return
}

func convert(path string, args string) {
    MyPrintf("start convert\n")
    selectArg := []string{"-i", path}
    allArgs := []string{"-threads", "8", "-vcodec", "libx264"}
    convertFileName := "c" + path
    if len(args) > 0 {
        argsSplit := strings.Split(args, " ")
        allArgs = append(allArgs, argsSplit...)
    }
    cmdLine := append(selectArg, allArgs...)
    cmdLine = append(cmdLine, convertFileName)
    //path   %d.mp4
    MyPrintf("input args[%v]\n", cmdLine)
    cmd := exec.Command("ffmpeg", cmdLine...)
    w := bytes.NewBuffer(nil)
    cmd.Stderr = w
    cmd.Run()
    //MyPrintf("%s\n", string(w.Bytes()))
    f, err := os.OpenFile("c"+path+".log", os.O_WRONLY|os.O_CREATE, 0655)
    defer f.Close()
    if err == nil {
        f.WriteString(string(w.Bytes()))
        MyPrintf("writelog success\n")
    } else {
        MyPrintf("write log error[%v]", err.Error())
    }
    fileName := strings.Split(path, ".")[0]
    MyPrintf("chan \n")
    converSuccess <- fileName
    MyPrintf("convert success\n")
    return
}

func uploadFile(name string, path string) {
    MyPrintf("start upload\n")
    ftp, err := goftp.Connect("188.166.213.154:21")
    if err != nil {
        panic(err)
    }
    if err = ftp.Login("video", "qpalzm"); err != nil {
        panic(err)
    }
    file, err := os.Open(name)
    if err != nil {
        panic(err)
    }
    if err := ftp.Stor(path+"/"+name[1:], file); err != nil {
        panic(err)
    }
    logfile, err := os.Open(name + ".log")
    if err != nil {
        panic(err)
    }
    if err := ftp.Stor(path+"/"+name+".log", logfile); err != nil {
        panic(err)
    }
    defer func() {
        ftp.Close()
        file.Close()
        logfile.Close()
        //转换出来的文件
        os.Remove(name)
        os.Remove(name + ".log")
        //转换前的文件
        os.Remove(name[1:])
        MyPrintf("upload over\n")
    }()
}

func MyPrintf(format string, a ...interface{}) (n int, err error) {
    if printLog {
        return fmt.Printf(format, a ...)
    }
    return
}
