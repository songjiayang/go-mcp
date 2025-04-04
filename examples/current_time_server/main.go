package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
)

var (
	mode string
	addr = "127.0.0.1:8080"
)

func main() {
	// new mcp server with stdio or sse transport
	srv, err := server.NewServer(
		getTransport(),
		server.WithServerInfo(protocol.Implementation{
			Name:    "current-time-v2-server",
			Version: "1.0.0",
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// new protocal tool with name, descipriton and properties
	tool := &protocol.Tool{
		Name:        "current time",
		Description: "Get current time with timezone, Asia/Shanghai is default",
		InputSchema: protocol.InputSchema{
			Type: protocol.Object,
			Properties: map[string]interface{}{
				"timezone": map[string]string{
					"type":        "string",
					"description": "current time timezone",
				},
			},
			Required: []string{"timezone"},
		},
	}

	// new tool handler and return result
	handler := func(request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		timezone, ok := request.Arguments["timezone"].(string)
		if !ok {
			return nil, errors.New("timezone must be a string")
		}

		loc, err := time.LoadLocation(timezone)
		if err != nil {
			return nil, fmt.Errorf("parse timezone with error: %v", err)
		}
		text := fmt.Sprintf(`current time is %s`, time.Now().In(loc))

		return &protocol.CallToolResult{
			Content: []protocol.Content{
				protocol.TextContent{
					Type: "text",
					Text: text,
				},
			},
		}, nil
	}

	// register tool and start mcp server
	srv.RegisterTool(tool, handler)
	// srv.RegisterResource()
	// srv.RegisterPrompt()
	// srv.RegisterResourceTemplate()

	errCh := make(chan error)
	go func() {
		errCh <- srv.Run()
	}()

	if err = signalWaiter(errCh); err != nil {
		log.Fatalf("signal waiter: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
}

func getTransport() (t transport.ServerTransport) {
	flag.StringVar(&mode, "transport", "stdio", "The transport to use, should be \"stdio\" or \"sse\"")
	flag.Parse()

	if mode == "stdio" {
		log.Println("start current time mcp server with stdio transport")
		t = transport.NewStdioServerTransport()
	} else {
		log.Printf("start current time mcp server with sse transport, listen %s", addr)
		t, _ = transport.NewSSEServerTransport(addr)
	}

	return t
}

func signalWaiter(errCh chan error) error {
	signalToNotify := []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM}
	if signal.Ignored(syscall.SIGHUP) {
		signalToNotify = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, signalToNotify...)

	select {
	case sig := <-signals:
		switch sig {
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM:
			log.Printf("Received signal: %s\n", sig)
			// graceful shutdown
			return nil
		}
	case err := <-errCh:
		return err
	}

	return nil
}
