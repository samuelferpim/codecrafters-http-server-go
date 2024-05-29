package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func HttpResponse(conn net.Conn, status string, body *string, contentType string) {
	mapStatus := map[string]string{
		"200": "200 OK",
		"404": "404 Not Found",
	}

	statusLine := "HTTP/1.1 " + mapStatus[status] + "\r\n"

	if body != nil {
		bodyLength := strconv.Itoa(len(*body))
		headers := "Content-Type: " + contentType + "\r\nContent-Length: " + bodyLength + "\r\n\r\n"
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

func Handler(conn net.Conn, directory string) {
	defer conn.Close()

	request, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		fmt.Println("Error reading request: ", err.Error())
		return
	}
	fmt.Printf("Request: %v\n", request)

	pathSegments, err := ProcessPath(request)
	if err != nil {
		HttpResponse(conn, "404", nil, "text/plain")
		return
	}

	if len(pathSegments) == 0 {
		HttpResponse(conn, "200", nil, "text/plain")
		return
	}

	switch pathSegments[0] {
	case "echo":
		if len(pathSegments) > 1 {
			HttpResponse(conn, "200", &pathSegments[1], "text/plain")
		} else {
			HttpResponse(conn, "200", nil, "text/plain")
		}
	case "user-agent":
		userAgentData := request.UserAgent()
		HttpResponse(conn, "200", &userAgentData, "text/plain")
	case "files":
		if len(pathSegments) > 1 {
			file := pathSegments[1]
			data, err := os.ReadFile(filepath.Join(directory, file))
			if err != nil {
				HttpResponse(conn, "404", nil, "text/plain")
				return
			}
			body := string(data)
			HttpResponse(conn, "200", &body, "application/octet-stream")
		} else {
			HttpResponse(conn, "404", nil, "text/plain")
		}
	default:
		HttpResponse(conn, "404", nil, "text/plain")
	}
}

func main() {
	directory := flag.String("directory", ".", "The directory to serve files from")
	flag.Parse()

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

		go Handler(conn, *directory)
	}
}
