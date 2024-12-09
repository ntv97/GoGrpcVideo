package main

import (
 "fmt"
 //"context"
 uploadpb "github.com/GoGrpcVideo/proto"
 "github.com/GoGrpcVideo/pkg/app"
 "google.golang.org/grpc"
 "google.golang.org/grpc/credentials"
 "crypto/tls"
 "log"
 //"net"
 "bytes"
 "os"
 "path/filepath"
 "io"
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

	fileName := filepath.Base(file.FilePath)
	return stream.SendAndClose(&uploadpb.FileUploadResponse{FileName: fileName, Size: fileSize})
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
        err = a.Run()
        if err != nil {
                log.Fatal(err)
        }
	serverCert, err := tls.LoadX509KeyPair("./cert/server-cert.pem", "./cert/server-key.pem")
        if err != nil {
                log.Fatalf("Failed to load server certificate: %v", err)
        }

        // Create a TLS configuration
        tlsConfig := &tls.Config{
                Certificates: []tls.Certificate{serverCert},
                //ClientAuth:   tls.RequireAndVerifyClientCert, // Optional: require client certificates
		ClientAuth:   tls.NoClientCert,
        }

        // Create a gRPC server with TLS credentials
        creds := credentials.NewTLS(tlsConfig)

	//fmt.Println("Listening on :50051") 
	//g := grpc.NewServer()
	g := grpc.NewServer(grpc.Creds(creds))
	uploadpb.RegisterFileServiceServer(g, &FileServiceServer{})
	if err := g.Serve(a.Listener); err != nil {
		fmt.Println("Serve error")
		return
	}

}
