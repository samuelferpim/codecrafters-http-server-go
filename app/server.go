package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	StatusOK                  = "200 OK"
	StatusCreated             = "201 Created"
	StatusNotFound            = "404 Not Found"
	StatusInternalServerError = "500 Internal Server Error"
	contentTypePlainText      = "text/plain"
	contentTypeOctetStream    = "application/octet-stream"
	bufferSize                = 4096
)

func HttpResponse(conn net.Conn, status string, body *[]byte, contentType string) {
	statusLine := "HTTP/1.1 " + status + "\r\n"
	headers := "Content-Type: " + contentType + "\r\n"
	if body != nil {
		bodyLength := strconv.Itoa(len(*body))
		headers += "Content-Length: " + bodyLength + "\r\n\r\n"
		conn.Write([]byte(statusLine + headers))
		conn.Write(*body)
	} else {
		headers += "\r\n"
		conn.Write([]byte(statusLine + headers))
	}
}

func GetPathSegments(request *http.Request) []string {
	return strings.FieldsFunc(request.URL.Path, func(r rune) bool {
		return r == '/'
	})
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
			return segments[:2], nil
		}
		return segments[:1], nil
	}

	return nil, errors.New("invalid path")
}

func Handler(conn net.Conn, directory string) {
	defer conn.Close()

	request, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		fmt.Println("Error reading request:", err)
		HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText)
		return
	}

	pathSegments, err := ProcessPath(request)
	if err != nil {
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText)
		return
	}

	if len(pathSegments) == 0 {
		HttpResponse(conn, StatusOK, nil, contentTypePlainText)
		return
	}

	switch pathSegments[0] {
	case "echo":
		handleEcho(conn, pathSegments)
	case "user-agent":
		handleUserAgent(conn, request)
	case "files":
		handleFiles(conn, request, directory, pathSegments)
	default:
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText)
	}
}

func handleEcho(conn net.Conn, pathSegments []string) {
	if len(pathSegments) > 1 {
		responseBody := []byte(pathSegments[1])
		HttpResponse(conn, StatusOK, &responseBody, contentTypePlainText)
	} else {
		HttpResponse(conn, StatusOK, nil, contentTypePlainText)
	}
}

func handleUserAgent(conn net.Conn, request *http.Request) {
	userAgentData := request.UserAgent()
	responseBody := []byte(userAgentData)
	HttpResponse(conn, StatusOK, &responseBody, contentTypePlainText)
}

func handleFiles(conn net.Conn, request *http.Request, directory string, pathSegments []string) {
	if len(pathSegments) < 2 {
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText)
		return
	}
	filePath := filepath.Join(directory, pathSegments[1])

	switch request.Method {
	case http.MethodGet:
		file, err := os.Open(filePath)
		if err != nil {
			HttpResponse(conn, StatusNotFound, nil, contentTypePlainText)
			return
		}
		defer file.Close()

		fileInfo, err := file.Stat()
		if err != nil {
			HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText)
			return
		}

		headers := "HTTP/1.1 " + StatusOK + "\r\n" +
			"Content-Type: " + contentTypeOctetStream + "\r\n" +
			"Content-Length: " + strconv.FormatInt(fileInfo.Size(), 10) + "\r\n\r\n"
		conn.Write([]byte(headers))

		buffer := make([]byte, bufferSize)
		for {
			n, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText)
				return
			}
			if n == 0 {
				break
			}
			conn.Write(buffer[:n])
		}

	case http.MethodPost:
		fileContents, err := io.ReadAll(request.Body)
		if err != nil {
			HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText)
			return
		}
		err = os.WriteFile(filePath, fileContents, 0644)
		if err != nil {
			HttpResponse(conn, StatusInternalServerError, nil, contentTypePlainText)
			return
		}
		HttpResponse(conn, StatusCreated, nil, contentTypePlainText)

	default:
		HttpResponse(conn, StatusNotFound, nil, contentTypePlainText)
	}
}

func main() {
	directory := flag.String("directory", ".", "The directory to serve files from")
	flag.Parse()

	listener, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221:", err)
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Println("Server is listening on port 4221")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go Handler(conn, *directory)
	}
}
