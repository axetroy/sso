package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/axetroy/local-ip"
	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/mholt/archiver"
	"github.com/mitchellh/ioprogress"
	"github.com/urfave/cli"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

var (
	downloading      bool
	downloadTimes    int
	shareFileAbsPath string

	// color printer
	red       = color.New(color.FgRed).SprintFunc()
	yellow    = color.New(color.FgYellow).SprintFunc()
	green     = color.New(color.FgGreen).SprintFunc()
	blue      = color.New(color.FgBlue).SprintFunc()
	cyan      = color.New(color.FgCyan).SprintFunc()
	underline = color.New(color.Underline).SprintFunc()
)

type Config struct {
	Host               string
	Port               string
	AutoClose          bool
	AllowDownloadTimes int
}

var config = Config{}

func main() {

	var (
		err error
	)

	defer func() {
		if err != nil {
			log.Fatal(err)
			return
		}
	}()

	ticker := time.NewTicker(time.Microsecond * 100)

	// start a ticker to check this download have done or not
	go func() {
		for range ticker.C {
			check()
		}
	}()

	app := cli.NewApp()

	app.Name = "sso"
	app.Usage = "sso <file/path>"
	app.Version = "0.2.0"
	app.Description = "Share your file/folder with command line interface."

	app.Flags = []cli.Flag{
		cli.Int64Flag{
			Name:  "port, p",
			Usage: "the port of server",
		},
		cli.BoolTFlag{
			Name:  "auto, a",
			Usage: "auto close server after file/folder been download",
		},
		cli.IntFlag{
			Name:  "download-times, dt",
			Usage: "allow this share download times",
			Value: 1,
		},
	}

	http.HandleFunc("/", handler)

	app.Action = action

	if err = app.Run(os.Args); err != nil {
		return
	}
}

func check() {
	if config.AutoClose && downloadTimes >= config.AllowDownloadTimes {
		fmt.Println(green("[SSO]: The file/folder have been download. close server."))
		os.Exit(0)
	}
}

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

func action(c *cli.Context) (err error) {
	var (
		ip            string
		cwd           string
		fileStat      os.FileInfo
		filePath      = c.Args().Get(0)
		port          = c.Int64("port")
		autoClose     = c.BoolT("auto")
		downloadTimes = c.Int("download-times")
		absFilePath   string
	)

	defer func() {
		if err != nil {
			fmt.Println(red("[SSO]: " + err.Error()))
			os.Exit(1)
			return
		}
	}()

	if err, ip = local_ip.Get(); err != nil {
		return
	}

	config.Host = ip
	config.AutoClose = autoClose
	config.AllowDownloadTimes = downloadTimes

	server := &http.Server{}

	if port == 0 {
		if portInt, err := getFreePort(); err != nil {
			return err
		} else {
			port = int64(portInt)
		}
	}

	config.Port = fmt.Sprintf("%v", port)

	server.Addr = ":" + config.Port

	if path.IsAbs(filePath) {
		absFilePath = filePath
	} else {
		if cwd, err = os.Getwd(); err != nil {
			return
		}
		absFilePath = path.Join(cwd, filePath)
	}

	shareFileAbsPath = absFilePath

	if fileStat, err = os.Stat(absFilePath); err != nil {
		return
	}

	downloadUrl := fmt.Sprintf("http://%v%v\n", ip, server.Addr)

	fileSize := humanize.Bytes(uint64(fileStat.Size()))

	fmt.Printf("[SSO]: The file/folder %v (%v) have been shared on %v", yellow(path.Base(absFilePath)), blue(fileSize), green(underline(downloadUrl)))

	fmt.Println(cyan("[SSO]: Once file/folder been downloaded, Service will shut down automatically."))

	if err = server.ListenAndServe(); err != nil {
		return
	}

	return
}

func handler(writer http.ResponseWriter, request *http.Request) {
	var (
		err      error
		fileStat os.FileInfo
	)

	defer func() {
		if err != nil {
			fmt.Println(err)
			_, _ = writer.Write([]byte(err.Error()))
		}
	}()

	check()

	if downloading {
		err = errors.New("the file is being downloading by others. please wait")
		return
	}

	defer func() {
		downloading = false
	}()

	if fileStat, err = os.Stat(shareFileAbsPath); err != nil {
		return
	}

	fmt.Println(blue("Downloading..."))

	downloading = true

	if fileStat.IsDir() {
		err = downloadFolder(writer)
	} else {
		err = downloadFile(writer)
	}

}

func downloadFile(writer http.ResponseWriter) (err error) {
	var (
		stat os.FileInfo
		file *os.File
	)
	if stat, err = os.Stat(shareFileAbsPath); err != nil {
		return
	}
	// add download headers
	writer.Header().Add("Content-Length", strconv.Itoa(int(stat.Size())))
	writer.Header().Add("content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", stat.Name()))

	file, err = os.Open(shareFileAbsPath)

	// Create the progress reader
	progress := &ioprogress.Reader{
		Reader: bufio.NewReader(file),
		Size:   stat.Size(),
	}

	n, err := io.Copy(writer, progress)

	if err != nil {
		return
	}

	downloadTimes++

	text := fmt.Sprintf("[SSO]: The file size %v have been downloaded.\n", humanize.Bytes(uint64(n)))

	fmt.Printf(green(text))
	return
}

func downloadFolder(writer http.ResponseWriter) (err error) {
	var (
		file *os.File
	)
	z := archiver.Tar{
		MkdirAll:               false,
		ContinueOnError:        false,
		OverwriteExisting:      false,
		ImplicitTopLevelFolder: false,
	}

	if err = z.Create(writer); err != nil {
		return
	}

	file, err = os.Open(shareFileAbsPath)

	if err != nil {
		return
	}

	defer func() {
		if er := file.Close(); er != nil {
			err = er
			return
		}
	}()

	if err != nil {
		return
	}

	if err != nil {
		return
	}

	defer func() {
		if er := z.Close(); er != nil {
			err = er
			return
		}
	}()

	baseDir := shareFileAbsPath

	rootDir := path.Base(baseDir)

	writer.Header().Add("content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", rootDir+".tar"))

	if err = writeFile(rootDir, baseDir, z); err != nil {
		return
	}

	downloadTimes++

	text := fmt.Sprintf("[SSO]: The directory have been downloaded.\n")

	fmt.Printf(green(text))

	return
}

func writeFile(rootDir, baseDir string, z archiver.Tar) (err error) {
	files, err := ioutil.ReadDir(baseDir)

	if err != nil {
		return
	}

	for _, fileInfo := range files {
		var (
			file         *os.File
			internalName string
		)
		filename := fileInfo.Name()
		// get file's name for the inside of the archive
		if internalName, err = archiver.NameInArchive(fileInfo, "", filename); err != nil {
			return
		}

		// open the file
		if file, err = os.Open(path.Join(baseDir, filename)); err != nil {
			return
		}

		internalName = path.Join(rootDir, internalName)

		// write it to the archive
		if err = z.Write(archiver.File{
			FileInfo: archiver.FileInfo{
				FileInfo:   fileInfo,
				CustomName: internalName,
			},
			ReadCloser: file,
		}); err != nil {
			return
		}

		err = file.Close()

		// 如果是目录的话，继续便利下一层目录
		if fileInfo.IsDir() {
			err = writeFile(path.Join(rootDir, filename), path.Join(baseDir, filename), z)
		}
	}

	return
}
