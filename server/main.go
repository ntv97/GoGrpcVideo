package main

import (
 "fmt"
 //"context"
 uploadpb "github.com/GoGrpcVideo/proto"
 "github.com/GoGrpcVideo/pkg/app"
 //"github.com/GoGrpcVideo/pkg/media"
 "google.golang.org/grpc"
 "log"
 "net"
 "net/http"
 "bytes"
 "strings"
 "os"
 "path/filepath"
 "io"
 "github.com/soheilhy/cmux"
)

type FileServiceServer struct {
	uploadpb.UnimplementedFileServiceServer
}

type File struct {
	FilePath   string
	buffer     *bytes.Buffer
	OutputFile *os.File
}

func NewFile() *File {
	return &File{
		buffer: &bytes.Buffer{},
	}
}

func (f *File) SetFile(fileName, path string) error {
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	f.FilePath = filepath.Join(path, fileName)
	file, err := os.Create(f.FilePath)
	if err != nil {
		return err
	}
	f.OutputFile = file
	return nil
}

func (f *File) Write(chunk []byte) error {
	if f.OutputFile == nil {
		return nil
	}
	_, err := f.OutputFile.Write(chunk)
	return err
}

func (f *File) Close() error {
	return f.OutputFile.Close()
}

func (g *FileServiceServer) Upload(stream uploadpb.FileService_UploadServer) error {
	file := NewFile()
	var fileSize uint32
	fileSize = 0
	defer func() {
		if err := file.OutputFile.Close(); err != nil {
			fmt.Println("Error 0")
			return
		}
	}()
	for {
		req, err := stream.Recv()
		if file.FilePath == "" {
			file.SetFile(req.GetFileName(), "./videosgrpc")
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error 1")
			return nil
			
		}
		chunk := req.GetChunk()
		fileSize += uint32(len(chunk))
		if err := file.Write(chunk); err != nil {
			fmt.Println("Error 2")
			return nil
		}
	}

	fmt.Println("grpc file path: ", file.FilePath)
	//app.APP.Queue = append(app.APP.Queue, file.FilePath)
	if err := app.APP.Library.Add(file.FilePath); err != nil {
		fmt.Println(err)
	}
	fmt.Println("after Add, sizeof playlist videos: ", len(app.APP.Library.Videos))
	fileName := filepath.Base(file.FilePath)
	if err := stream.SendAndClose(&uploadpb.FileUploadResponse{FileName: fileName, Size: fileSize}); err != nil {
		return err
	}
	//go app.APP.Library.Add(file.FilePath)
	//return stream.SendAndClose(&uploadpb.FileUploadResponse{FileName: fileName, Size: fileSize})
	return nil
}

// GRPC Server initialisation
func serveGRPC(l net.Listener) {
        g := grpc.NewServer()
        uploadpb.RegisterFileServiceServer(g, &FileServiceServer{})
        if err := g.Serve(l); err != nil {
                fmt.Println("Serve error")
                return
        }

}

func serveHTTP(l net.Listener, a *app.App) error {
	s := &http.Server{
		Handler: a.Router,
	}
	if err := s.Serve(l); err != cmux.ErrListenerClosed {
		log.Fatalf("error serving HTTP : %+v", err)
		return err
	}
	return nil
}



func main() {
	cfg := app.DefaultConfig()
	err := cfg.ReadFile("config.json")
        if err != nil && !os.IsNotExist(err) {
                log.Fatal(err)
        }
        a, err := app.NewApp(cfg)
        if err != nil {
                log.Fatal(err)
        }
        addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
        log.Printf("Local server: http://%s", addr)
        //err = a.Run()
        if err != nil {
                log.Fatal(err)
        }
	app.APP = a
	m := cmux.New(a.Listener)
        httpListener := m.Match(cmux.HTTP1Fast())
        grpclistener := m.Match(cmux.Any())
        go serveGRPC(grpclistener)
        go serveHTTP(httpListener, a)
        if err := m.Serve(); !strings.Contains(err.Error(), "use of closed network connection") {
                log.Fatalf("MUX ERR : %+v", err)
        }
}
