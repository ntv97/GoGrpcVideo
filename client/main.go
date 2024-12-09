package main

import (
	"fmt"
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	uploadpb "github.com/GoGrpcVideo/proto"
	"google.golang.org/grpc"
	"crypto/tls"
        "crypto/x509"
        "google.golang.org/grpc/credentials"
        "io/ioutil"
)

type ClientService struct {
	addr      string
	filePath  string
	batchSize int
	client    uploadpb.FileServiceClient
}

func New(addr string, filePath string, batchSize int) *ClientService {
	return &ClientService{
		addr:      addr,
		filePath:  filePath,
		batchSize: batchSize,
	}
}

func (s *ClientService) SendFile() error {
	log.Println(s.addr, s.filePath)
	caCert, err := ioutil.ReadFile("./cert/ca-cert.pem")
        if err != nil {
                log.Fatalf("Failed to read CA certificate: %v", err)
        }
        caCertPool := x509.NewCertPool()
        caCertPool.AppendCertsFromPEM(caCert)

        // Create a TLS configuration
        tlsConfig := &tls.Config{
                RootCAs: caCertPool,
        }

        // Create a gRPC connection with TLS credentials
        creds := credentials.NewTLS(tlsConfig)
	//conn, err := grpc.Dial(s.addr, grpc.WithInsecure())
	conn, err := grpc.Dial(s.addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return err
	}
	defer conn.Close()
	s.client = uploadpb.NewFileServiceClient(conn)
	interrupt := make(chan os.Signal, 1)
	shutdownSignals := []os.Signal{
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGQUIT,
	}
	signal.Notify(interrupt, shutdownSignals...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(s *ClientService) {
		if err = s.upload(ctx, cancel); err != nil {
			log.Fatal(err)
			cancel()
		}
	}(s)

	select {
	case killSignal := <-interrupt:
		log.Println("Got ", killSignal)
		cancel()
	case <-ctx.Done():
	}
	return nil
}

func (s *ClientService) upload(ctx context.Context, cancel context.CancelFunc) error {
	stream, err := s.client.Upload(ctx)
	if err != nil {
		return err
	}
	file, err := os.Open(s.filePath)
	if err != nil {
		return err
	}
	buf := make([]byte, s.batchSize)
	batchNumber := 1
	for {
		num, err := file.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		chunk := buf[:num]

		if err := stream.Send(&uploadpb.FileUploadRequest{FileName: s.filePath, Chunk: chunk}); err != nil {
			return err
		}
		log.Printf("Sent - batch #%v - size - %v\n", batchNumber, len(chunk))
		batchNumber += 1

	}
	res, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}
	log.Printf("Sent - %v bytes - %s\n", res.GetSize(), res.GetFileName())
	cancel()
	return nil
}

func main() {
	fmt.Println("Client started")
	serverAddr := "0.0.0.0:38457"
	filePath := "./dogs.mp4"
	batchSize := 1024*1024
	clientService := New(serverAddr, filePath, batchSize)
	if err := clientService.SendFile(); err != nil {
		log.Fatal(err)
	}
}
