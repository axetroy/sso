package main

import (
  "bufio"
  "errors"
  "fmt"
  "github.com/axetroy/local-ip"
  "github.com/dustin/go-humanize"
  "github.com/fatih/color"
  "github.com/mitchellh/ioprogress"
  "github.com/urfave/cli"
  "io"
  "net"
  "net/http"
  "os"
  "path"
  "strconv"
  "time"
)

func getFreePort() (port int, err error) {
  listener, err := net.Listen("tcp", "127.0.0.1:0")
  if err != nil {
    return 0, err
  }
  defer listener.Close()

  addr := listener.Addr().String()
  _, portString, err := net.SplitHostPort(addr)
  if err != nil {
    return 0, err
  }

  return strconv.Atoi(portString)
}

var (
  downloading      bool
  downloadTimes    int
  shareFileAbsPath string
  ip               string
  err              error
  fileSize         int64

  // color printer
  red       = color.New(color.FgRed).SprintFunc()
  yellow    = color.New(color.FgYellow).SprintFunc()
  green     = color.New(color.FgGreen).SprintFunc()
  blue      = color.New(color.FgBlue).SprintFunc()
  cyan      = color.New(color.FgCyan).SprintFunc()
  underline = color.New(color.Underline).SprintFunc()
)

func check() {
  if downloadTimes > 0 {
    fmt.Println(green("[SSO]: The file have been download. close server."))
    os.Exit(0)
  }
}

func main() {

  ticker := time.NewTicker(time.Microsecond * 100)

  // start a ticker to check this download have done or not
  go func() {
    for range ticker.C {
      check()
    }
  }()

  if err, ip = local_ip.Get(); err != nil {
    panic(err)
    return
  }

  app := cli.NewApp()

  app.Name = "sso"
  app.Usage = "sso your_file_path_that_you_want_to_share"
  app.Version = "0.1.0"
  app.Description = "Share your file with command line interface."

  app.Flags = []cli.Flag{
    cli.Int64Flag{
      Name:  "port, p",
      Usage: "The port of server",
    },
  }

  server := &http.Server{}

  http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
    check()

    if downloading {
      if _, err := writer.Write([]byte("The file is being downloading by others. please wait!")); err != nil {
        return
      }
      return
    }

    defer func() {
      downloading = false
    }()

    fmt.Println(blue("Downloading..."))

    downloading = true

    file, err := os.Open(shareFileAbsPath)

    if err != nil {
      return
    }

    defer file.Close()

    if err != nil {
      fmt.Println(err)
      return
    }

    stat, err := file.Stat()

    if err != nil {
      fmt.Println(err)
      return
    }

    // add download headers
    writer.Header().Add("Content-Length", strconv.Itoa(int(stat.Size())))
    writer.Header().Add("content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", stat.Name()))

    // Create the progress reader
    progress := &ioprogress.Reader{
      Reader: bufio.NewReader(file),
      Size:   stat.Size(),
    }

    n, err := io.Copy(writer, progress)

    if err != nil {
      fmt.Println(err)
      return
    }

    downloadTimes++

    text := fmt.Sprintf("[SSO]: The file size %v have been downloaded.\n", humanize.Bytes(uint64(n)))

    fmt.Printf(green(text))
  })

  app.Action = func(c *cli.Context) (err error) {
    defer func() {
      if err != nil {
        fmt.Println(red("[SSO]: " + err.Error()))
        os.Exit(1)
        return
      }
    }()

    var (
      cwd         string
      fileStat    os.FileInfo
      filePath    = c.Args().Get(0)
      port        = c.Int64("port")
      absFilePath string
    )

    if port == 0 {
      if portInt, err := getFreePort(); err != nil {
        return err
      } else {
        port = int64(portInt)
      }
    }

    server.Addr = ":" + strconv.Itoa(int(port))

    if cwd, err = os.Getwd(); err != nil {
      return
    }

    if path.IsAbs(filePath) {
      absFilePath = filePath
    } else {
      absFilePath = path.Join(cwd, filePath)
    }

    shareFileAbsPath = absFilePath

    if fileStat, err = os.Stat(absFilePath); err != nil {
      return
    }

    if fileStat.IsDir() {
      err = errors.New("can only specify shared file, not shared directories")
      return
    }

    downloadUrl := fmt.Sprintf("http://%v%v\n", ip, server.Addr)

    fileSize := humanize.Bytes(uint64(fileStat.Size()))

    fmt.Printf("[SSO]: The file %v (%v) have been shared on %v", yellow(path.Base(absFilePath)), blue(fileSize), green(underline(downloadUrl)))

    fmt.Println(cyan("[SSO]: Once file been downloaded, Service will shut down automatically."))

    if err = server.ListenAndServe(); err != nil {
      return
    }

    return
  }

  app.Run(os.Args)
}
