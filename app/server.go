package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func HttpResponse(conn net.Conn, status string, body *string) {
	mapStatus := map[string]string{
		"200": "200 OK",
		"404": "404 Not Found",
	}

	statusLine := "HTTP/1.1 " + mapStatus[status] + "\r\n"

	if body != nil {
		bodyLength := strconv.Itoa(len(*body))
		headers := "Content-Type: text/plain\r\nContent-Length: " + bodyLength + "\r\n\r\n"
		conn.Write([]byte(statusLine + headers + *body))
	} else {
		conn.Write([]byte(statusLine + "\r\n"))
	}
}

func GetPathSegments(request *http.Request) []string {
	path := request.URL.Path

	segments := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/'
	})

	return segments
}

func ProcessPath(request *http.Request) ([]string, error) {
	segments := GetPathSegments(request)

	validPaths := map[string]struct{}{
		"echo":       {},
		"user-agent": {},
		"files":      {},
	}

	if len(segments) == 0 {
		return []string{}, nil
	}

	if _, exists := validPaths[segments[0]]; exists {
		if len(segments) > 1 {
			nextSegment := segments[1]
			return []string{segments[0], nextSegment}, nil
		}
		return []string{segments[0]}, nil
	}

	return []string{}, errors.New("invalid path")
}

func Handler(conn net.Conn) {
	defer conn.Close()

	request, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		fmt.Println("Error reading request: ", err.Error())
		return
	}
	fmt.Printf("Request: %v\n", request)

	pathSegments, err := ProcessPath(request)
	if err != nil {
		HttpResponse(conn, "404", nil)
		return
	}

	if len(pathSegments) == 0 {
		HttpResponse(conn, "200", nil)
		return
	}

	switch pathSegments[0] {
	case "echo":
		if len(pathSegments) > 1 {
			HttpResponse(conn, "200", &pathSegments[1])
		} else {
			HttpResponse(conn, "200", nil)
		}
	case "user-agent":
		userAgentData := request.UserAgent()
		HttpResponse(conn, "200", &userAgentData)
	case "files":
		if len(pathSegments) > 1 {
			file := pathSegments[1]
			dir := os.Args[1] // Assume the directory is passed as the first argument
			data, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				response := "HTTP/1.1 404 Not Found\r\n\r\n"
				conn.Write([]byte(response))
				return
			}
			response := "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: " + strconv.Itoa(len(data)) + "\r\n\r\n" + string(data) + "\r\n"
			conn.Write([]byte(response))
		} else {
			HttpResponse(conn, "404", nil)
		}
	default:
		HttpResponse(conn, "404", nil)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run server.go <directory>")
		os.Exit(1)
	}

	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}

		go Handler(conn)
	}
}
